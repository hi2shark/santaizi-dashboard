package rpc

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
	telemetryservice "github.com/hi2shark/santaizi-dashboard/service/telemetry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm/clause"
)

type PrimaryCollectorHandler struct {
	pb.UnimplementedSantaiziCollectorServiceServer
	pb.UnimplementedSantaiziReplicationServiceServer
	signer *telemetryservice.Signer
	store  *telemetryservice.Store
}

func NewPrimaryCollectorHandler() (*PrimaryCollectorHandler, error) {
	signer, err := telemetryservice.LoadOrCreateSigner(singleton.Conf.Telemetry.SigningKeyPath)
	if err != nil {
		return nil, err
	}
	return &PrimaryCollectorHandler{
		signer: signer,
		store:  telemetryservice.NewStoreWithBucketSize(singleton.DB, time.Duration(singleton.Conf.Telemetry.AvailabilityBucketSeconds)*time.Second),
	}, nil
}

func (h *PrimaryCollectorHandler) Register(ctx context.Context, request *pb.RegisterCollectorRequest) (*pb.RegisterCollectorResponse, error) {
	collector, err := findCollectorByToken(ctx, request.GetRegistrationToken())
	if err != nil {
		return nil, err
	}
	return &pb.RegisterCollectorResponse{
		CollectorUuid: collector.CollectorUUID, PrimaryPublicKey: h.signer.PublicKey(), KeyId: h.signer.KeyID(),
		ConfigVersion: collector.ConfigVersion,
	}, nil
}

func (h *PrimaryCollectorHandler) Sync(stream grpc.BidiStreamingServer[pb.CollectorSyncRequest, pb.CollectorSyncResponse]) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	hello := first.GetHello()
	if hello == nil {
		return status.Error(codes.InvalidArgument, "collector sync hello is required")
	}
	collector, err := findCollectorByToken(stream.Context(), hello.GetRegistrationToken())
	if err != nil {
		return err
	}
	if collector.CollectorUUID != hello.GetCollectorUuid() {
		return status.Error(codes.PermissionDenied, "collector token identity mismatch")
	}
	if err := h.sendCollectorConfig(stream, collector); err != nil {
		return err
	}
	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if runtime := request.GetRuntime(); runtime != nil {
			if err := saveCollectorRuntime(stream.Context(), collector.CollectorUUID, runtime, time.Now()); err != nil {
				return err
			}
			var current model.Collector
			if err := singleton.DB.WithContext(stream.Context()).First(&current, "collector_uuid = ?", collector.CollectorUUID).Error; err != nil {
				return err
			}
			if current.ConfigVersion != collector.ConfigVersion {
				collector = &current
				if err := h.sendCollectorConfig(stream, collector); err != nil {
					return err
				}
			} else if err := stream.Send(&pb.CollectorSyncResponse{Body: &pb.CollectorSyncResponse_Accepted{Accepted: true}}); err != nil {
				return err
			}
		}
	}
}

func (h *PrimaryCollectorHandler) sendCollectorConfig(stream grpc.BidiStreamingServer[pb.CollectorSyncRequest, pb.CollectorSyncResponse], collector *model.Collector) error {
	var rows []model.ObserverAssignment
	if err := singleton.DB.WithContext(stream.Context()).Where("observer_id = ?", collector.CollectorUUID).Find(&rows).Error; err != nil {
		return err
	}
	config := &pb.CollectorAuthorizationConfig{
		ConfigVersion: collector.ConfigVersion, PrimaryPublicKey: h.signer.PublicKey(), KeyId: h.signer.KeyID(),
	}
	for _, row := range rows {
		config.Assignments = append(config.Assignments, &pb.NodeAssignment{
			NodeUuid: row.NodeUUID, ObserverId: row.ObserverID, ValidFromUnixNano: row.ValidFrom,
			ValidToUnixNano: row.ValidTo, Generation: row.Generation, ConfigVersion: row.ConfigVersion,
		})
	}
	return stream.Send(&pb.CollectorSyncResponse{Body: &pb.CollectorSyncResponse_Config{Config: config}})
}

func (h *PrimaryCollectorHandler) Replicate(stream grpc.BidiStreamingServer[pb.ReplicationBatch, pb.ReplicationAck]) error {
	token := collectorTokenFromMetadata(stream.Context())
	collector, err := findCollectorByToken(stream.Context(), token)
	if err != nil {
		return err
	}
	for {
		batch, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if batch.GetCollectorUuid() != collector.CollectorUUID {
			return status.Error(codes.PermissionDenied, "replication collector identity mismatch")
		}
		committed, err := h.store.Replicate(stream.Context(), batch, time.Now())
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		if err := stream.Send(&pb.ReplicationAck{
			CollectorUuid: collector.CollectorUUID, ReplicationSession: batch.GetReplicationSession(),
			BatchSequence: batch.GetBatchSequence(), CommittedSpoolThroughId: committed,
		}); err != nil {
			return err
		}
	}
}

func findCollectorByToken(ctx context.Context, token string) (*model.Collector, error) {
	if token == "" {
		return nil, status.Error(codes.Unauthenticated, "collector token is required")
	}
	var collectors []model.Collector
	if err := singleton.DB.WithContext(ctx).Where("revoked = ? AND deleted = ?", false, false).Find(&collectors).Error; err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	for index := range collectors {
		if telemetryservice.RegistrationTokenMatches(token, collectors[index].TokenHash) {
			return &collectors[index], nil
		}
	}
	return nil, status.Error(codes.Unauthenticated, "collector token is invalid or revoked")
}

func collectorTokenFromMetadata(ctx context.Context) string {
	values, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	tokens := values.Get("collector_token")
	if len(tokens) == 0 {
		return ""
	}
	return tokens[0]
}

func saveCollectorRuntime(ctx context.Context, collectorUUID string, runtime *pb.CollectorRuntime, now time.Time) error {
	row := model.CollectorRuntime{
		CollectorUUID: collectorUUID, Status: "online", LastSeen: now.UnixNano(),
		SpoolSize: runtime.GetSpoolSize(), PendingRecords: runtime.GetPendingRecords(), OldestPending: runtime.GetOldestPendingUnixNano(),
		ReplicationCursor: runtime.GetReplicationCursor(), ConnectedAgents: runtime.GetConnectedAgents(), ProtocolVersion: runtime.GetProtocolVersion(),
	}
	return singleton.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "collector_uuid"}}, UpdateAll: true,
	}).Create(&row).Error
}
