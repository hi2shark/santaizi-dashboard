package collector

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"github.com/hi2shark/santaizi-dashboard/service/telemetry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const collectorProtocolVersion = "2"

type Runtime struct {
	pb.UnimplementedSantaiziTelemetryServiceServer
	pb.UnimplementedSantaiziCollectorServiceServer

	store  *Store
	config model.CollectorModeConfig
	grace  time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu                 sync.RWMutex
	collectorUUID      string
	processSession     string
	replicationSession []byte
	nextBatchSequence  uint64
	lastReplicationAck uint64
	connectedAgents    atomic.Uint64
}

func NewRuntime(parent context.Context, store *Store, config model.CollectorModeConfig, grace time.Duration) (*Runtime, error) {
	if store == nil || config.PrimaryEndpoint == "" || config.RegistrationToken == "" {
		return nil, errors.New("collector primary endpoint and registration token are required")
	}
	processID := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, processID); err != nil {
		return nil, err
	}
	replicationID := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, replicationID); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	runtime := &Runtime{
		store: store, config: config, grace: grace, ctx: ctx, cancel: cancel,
		processSession: hex.EncodeToString(processID), replicationSession: replicationID, nextBatchSequence: 1,
	}
	if cache, err := store.Authorization(ctx); err == nil {
		runtime.collectorUUID = cache.CollectorUUID
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		cancel()
		return nil, err
	}
	return runtime, nil
}

func (r *Runtime) Start() {
	r.wg.Add(3)
	go r.syncLoop()
	go r.replicationLoop()
	go r.healthLoop()
}

func (r *Runtime) Close() {
	r.cancel()
	r.wg.Wait()
}

func (r *Runtime) syncLoop() {
	defer r.wg.Done()
	for {
		if r.ctx.Err() != nil {
			return
		}
		if err := r.syncOnce(); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Printf("SANTAIZI>> collector sync disconnected: %v\n", err)
		}
		select {
		case <-r.ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func (r *Runtime) syncOnce() error {
	conn, err := grpc.NewClient(r.config.PrimaryEndpoint, r.dialOptions()...)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := pb.NewSantaiziCollectorServiceClient(conn)
	r.mu.RLock()
	collectorUUID := r.collectorUUID
	r.mu.RUnlock()
	if collectorUUID == "" {
		response, err := client.Register(r.ctx, &pb.RegisterCollectorRequest{RegistrationToken: r.config.RegistrationToken, ProtocolVersion: collectorProtocolVersion})
		if err != nil {
			return err
		}
		collectorUUID = response.GetCollectorUuid()
		if err := r.store.SaveAuthorization(r.ctx, collectorUUID, &pb.CollectorAuthorizationConfig{
			ConfigVersion: response.GetConfigVersion(), PrimaryPublicKey: response.GetPrimaryPublicKey(), KeyId: response.GetKeyId(),
		}, time.Now()); err != nil {
			return err
		}
		r.mu.Lock()
		r.collectorUUID = collectorUUID
		r.mu.Unlock()
	}
	stream, err := client.Sync(r.ctx)
	if err != nil {
		return err
	}
	cache, err := r.store.Authorization(r.ctx)
	if err != nil {
		return err
	}
	if err := stream.Send(&pb.CollectorSyncRequest{Body: &pb.CollectorSyncRequest_Hello{Hello: &pb.CollectorSyncHello{
		CollectorUuid: collectorUUID, RegistrationToken: r.config.RegistrationToken,
		CurrentConfigVersion: cache.ConfigVersion, Runtime: r.runtimeSnapshot(),
	}}}); err != nil {
		return err
	}
	recv := make(chan *pb.CollectorSyncResponse)
	recvErr := make(chan error, 1)
	go func() {
		for {
			response, err := stream.Recv()
			if err != nil {
				recvErr <- err
				return
			}
			select {
			case recv <- response:
			case <-r.ctx.Done():
				return
			}
		}
	}()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case err := <-recvErr:
			return err
		case response := <-recv:
			if config := response.GetConfig(); config != nil {
				if err := r.store.SaveAuthorization(r.ctx, collectorUUID, config, time.Now()); err != nil {
					return err
				}
			} else if response.GetAccepted() {
				if err := r.store.TouchPrimarySeen(r.ctx, time.Now()); err != nil {
					return err
				}
			}
		case <-ticker.C:
			if err := stream.Send(&pb.CollectorSyncRequest{Body: &pb.CollectorSyncRequest_Runtime{Runtime: r.runtimeSnapshot()}}); err != nil {
				return err
			}
		}
	}
}

func (r *Runtime) replicationLoop() {
	defer r.wg.Done()
	for {
		if r.ctx.Err() != nil {
			return
		}
		if err := r.replicationOnce(); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Printf("SANTAIZI>> collector replication disconnected: %v\n", err)
		}
		select {
		case <-r.ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func (r *Runtime) replicationOnce() error {
	r.mu.RLock()
	collectorUUID := r.collectorUUID
	r.mu.RUnlock()
	if collectorUUID == "" {
		return errors.New("collector is not registered")
	}
	conn, err := grpc.NewClient(r.config.PrimaryEndpoint, append(r.dialOptions(), grpc.WithPerRPCCredentials(&collectorTokenCredential{token: r.config.RegistrationToken}))...)
	if err != nil {
		return err
	}
	defer conn.Close()
	stream, err := pb.NewSantaiziReplicationServiceClient(conn).Replicate(r.ctx)
	if err != nil {
		return err
	}
	r.mu.RLock()
	session := append([]byte(nil), r.replicationSession...)
	r.mu.RUnlock()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case <-ticker.C:
			r.mu.RLock()
			batchSequence := r.nextBatchSequence
			r.mu.RUnlock()
			outbox, err := r.store.ReadOutbox(r.ctx, 512)
			if err != nil {
				return err
			}
			if outbox.Through == 0 {
				continue
			}
			batch := &pb.ReplicationBatch{
				CollectorUuid: collectorUUID, ReplicationSession: session, BatchSequence: batchSequence,
				SpoolThroughId: outbox.Through, Events: outbox.Events, Observations: outbox.Observations,
				Gaps: outbox.Gaps, Health: outbox.Health, Runtime: r.runtimeSnapshot(), DataLoss: outbox.DataLoss,
			}
			if err := stream.Send(batch); err != nil {
				return err
			}
			ack, err := stream.Recv()
			if err != nil {
				return err
			}
			if ack.GetError() != "" {
				return errors.New(ack.GetError())
			}
			if ack.GetCollectorUuid() != collectorUUID || !subtleBytesEqual(ack.GetReplicationSession(), session) || ack.GetBatchSequence() != batchSequence {
				return errors.New("replication ACK identity mismatch")
			}
			if err := r.store.CommitReplicationAck(r.ctx, ack.GetCommittedSpoolThroughId()); err != nil {
				return err
			}
			r.mu.Lock()
			r.lastReplicationAck = ack.GetCommittedSpoolThroughId()
			r.nextBatchSequence++
			r.mu.Unlock()
		}
	}
}

func (r *Runtime) healthLoop() {
	defer r.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case sampledAt := <-ticker.C:
			r.mu.RLock()
			observerID := r.collectorUUID
			r.mu.RUnlock()
			if observerID == "" {
				continue
			}
			if err := r.store.RecordHealth(r.ctx, &pb.ObserverHealthSample{
				ObserverId: observerID, SampledAtUnixNano: sampledAt.UnixNano(), Healthy: true, ProcessSession: r.processSession,
			}); err != nil {
				fmt.Printf("SANTAIZI>> record collector health: %v\n", err)
			}
			if err := r.store.EnforceSpoolPolicy(r.ctx, observerID, r.config.SpoolMaxBytes,
				time.Duration(r.config.SpoolMaxAgeDays)*24*time.Hour, sampledAt); err != nil {
				fmt.Printf("SANTAIZI>> enforce collector spool policy: %v\n", err)
			}
		}
	}
}

func (r *Runtime) Ingest(stream grpc.BidiStreamingServer[pb.TelemetryRequest, pb.TelemetryResponse]) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	hello := first.GetHello()
	if hello == nil || len(hello.GetNodeUuid()) != 16 {
		return status.Error(codes.InvalidArgument, "collector telemetry hello is required")
	}
	r.mu.RLock()
	collectorUUID := r.collectorUUID
	r.mu.RUnlock()
	if collectorUUID == "" || hello.GetEndpointId() != collectorUUID {
		return status.Error(codes.PermissionDenied, "telemetry endpoint identity mismatch")
	}
	authorized, err := r.store.IsNodeAuthorized(stream.Context(), hello.GetNodeUuid(), time.Now())
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if !authorized {
		return status.Error(codes.PermissionDenied, "node is not assigned to this collector")
	}
	cache, err := r.store.Authorization(stream.Context())
	if err != nil {
		return status.Error(codes.Unavailable, "collector authorization cache is unavailable")
	}
	verification, err := telemetry.VerifyCredential(cache.PrimaryPublicKey, cache.KeyID, hello.GetCredential(), time.Now(), r.grace, authorized)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}
	if !subtleBytesEqual(verification.Claims.GetNodeUuid(), hello.GetNodeUuid()) {
		return status.Error(codes.PermissionDenied, "credential node mismatch")
	}
	r.connectedAgents.Add(1)
	defer r.connectedAgents.Add(^uint64(0))
	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if batch := request.GetBatch(); batch != nil {
			result, err := r.store.Ingest(stream.Context(), batch, collectorUUID, time.Now())
			if err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
			if err := stream.Send(&pb.TelemetryResponse{Acks: result.Acks}); err != nil {
				return err
			}
		}
		// Realtime snapshots are deliberately not ACKed and do not advance the
		// reliable cursor. The Agent keeps a direct Primary realtime path.
	}
}

func (r *Runtime) GetStatus(ctx context.Context, request *pb.CollectorStatusRequest) (*pb.CollectorStatus, error) {
	if r.config.StatusAuthorization == "" || subtle.ConstantTimeCompare([]byte(request.GetAuthorization()), []byte(r.config.StatusAuthorization)) != 1 {
		return nil, status.Error(codes.Unauthenticated, "collector status authorization failed")
	}
	runtime := r.runtimeSnapshot()
	return &pb.CollectorStatus{
		ConnectedAgents: runtime.GetConnectedAgents(), SpoolSize: runtime.GetSpoolSize(), PendingRecords: runtime.GetPendingRecords(),
		OldestPendingUnixNano: runtime.GetOldestPendingUnixNano(), ReplicationCursor: runtime.GetReplicationCursor(),
		LastPrimarySeenUnixNano: runtime.GetLastPrimarySeenUnixNano(), ProtocolVersion: runtime.GetProtocolVersion(),
	}, nil
}

func (r *Runtime) runtimeSnapshot() *pb.CollectorRuntime {
	stats, _ := r.store.RuntimeStats(r.ctx)
	cache, _ := r.store.Authorization(r.ctx)
	r.mu.RLock()
	collectorUUID := r.collectorUUID
	replicationCursor := r.lastReplicationAck
	r.mu.RUnlock()
	runtime := &pb.CollectorRuntime{
		CollectorUuid: collectorUUID, SampledAtUnixNano: time.Now().UnixNano(), SpoolSize: stats.SpoolBytes,
		PendingRecords: stats.Pending, OldestPendingUnixNano: stats.OldestPending, ReplicationCursor: replicationCursor,
		ConnectedAgents: r.connectedAgents.Load(), ProtocolVersion: collectorProtocolVersion,
	}
	if cache != nil {
		runtime.LastPrimarySeenUnixNano = cache.LastPrimarySeenNano
	}
	return runtime
}

func (r *Runtime) dialOptions() []grpc.DialOption {
	if r.config.PrimaryTLS {
		return []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12, InsecureSkipVerify: r.config.PrimaryInsecureTLS,
		}))}
	}
	return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
}

type collectorTokenCredential struct{ token string }

func (c *collectorTokenCredential) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"collector_token": c.token}, nil
}

func (c *collectorTokenCredential) RequireTransportSecurity() bool { return false }

func subtleBytesEqual(left, right []byte) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare(left, right) == 1
}
