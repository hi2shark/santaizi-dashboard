package telemetry

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// BenchmarkSyntheticTelemetry1000x10 models one five-second State round from
// 1000 Agents observed by 10 Collectors. The dimensions can be reduced for
// profiling with SANTAIZI_BENCH_AGENTS and SANTAIZI_BENCH_COLLECTORS.
//
// Run the acceptance-sized case explicitly:
//
//	go test -run '^$' -bench BenchmarkSyntheticTelemetry1000x10 -benchtime=1x ./service/telemetry
func BenchmarkSyntheticTelemetry1000x10(b *testing.B) {
	agents := benchmarkDimension("SANTAIZI_BENCH_AGENTS", 1000)
	collectors := benchmarkDimension("SANTAIZI_BENCH_COLLECTORS", 10)
	db, err := gorm.Open(sqlite.Open("file:"+b.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.TelemetryEvent{}, &model.TelemetryObservation{}, &model.TelemetryGap{},
		&model.ObserverHealthBucket{}, &model.ObserverPathBucket{}, &model.ObserverAssignment{},
		&model.AvailabilityRecomputeQueue{}, &model.CollectorReplicationReceipt{}, &model.CollectorRuntime{},
		&model.TelemetryDataLoss{},
	); err != nil {
		b.Fatal(err)
	}
	store := NewStore(db)
	b.ReportAllocs()
	b.ReportMetric(float64(agents), "agents/op")
	b.ReportMetric(float64(collectors), "collectors/op")
	b.ReportMetric(5, "sample_interval_s")
	b.ReportMetric(float64(agents*collectors), "observations/op")

	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		batches := syntheticReplicationRound(b, agents, collectors, iteration)
		goroutinesBefore := runtime.NumGoroutine()
		b.StartTimer()
		for _, batch := range batches {
			if _, err := store.Replicate(context.Background(), batch, time.Unix(2_000_000_000, 0)); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		if growth := runtime.NumGoroutine() - goroutinesBefore; growth > 2 {
			b.Fatalf("unexpected goroutine growth in one round: %d", growth)
		}
		var events, observations, receipts int64
		if err := db.Model(&model.TelemetryEvent{}).Count(&events).Error; err != nil {
			b.Fatal(err)
		}
		if err := db.Model(&model.TelemetryObservation{}).Count(&observations).Error; err != nil {
			b.Fatal(err)
		}
		if err := db.Model(&model.CollectorReplicationReceipt{}).Count(&receipts).Error; err != nil {
			b.Fatal(err)
		}
		if events != int64(agents) || observations != int64(agents*collectors) || receipts != int64(collectors) {
			b.Fatalf("events=%d observations=%d receipts=%d", events, observations, receipts)
		}
		for _, table := range []string{
			"telemetry_observations", "telemetry_events", "observer_path_buckets", "observer_health_buckets",
			"availability_recompute_queues", "collector_replication_receipts",
		} {
			if err := db.Exec("DELETE FROM " + table).Error; err != nil { // table names are a fixed internal allowlist
				b.Fatal(err)
			}
		}
	}
}

func syntheticReplicationRound(b *testing.B, agents, collectors, iteration int) []*pb.ReplicationBatch {
	b.Helper()
	now := time.Unix(2_000_000_000, 0)
	events := make([]*pb.TelemetryEvent, 0, agents)
	for agentIndex := 0; agentIndex < agents; agentIndex++ {
		nodeUUID := benchmarkID(1, uint64(agentIndex+1))
		sessionID := benchmarkID(uint64(iteration+2), uint64(agentIndex+1))
		eventID, err := EventID(nodeUUID, sessionID, 1)
		if err != nil {
			b.Fatal(err)
		}
		events = append(events, &pb.TelemetryEvent{
			EventId: eventID, NodeUuid: nodeUUID, SessionId: sessionID, Sequence: 1,
			EventType:           pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_STATE,
			Priority:            pb.TelemetryPriority_TELEMETRY_PRIORITY_P2_NORMAL,
			CollectedAtUnixNano: now.UnixNano(), AgentUptimeNano: uint64(time.Hour),
			SessionElapsedNano: uint64(5 * time.Second), ProtocolVersion: 2,
			SourceProtocol: pb.SourceProtocol_SOURCE_PROTOCOL_SANTAIZI_V2,
			Reliability:    pb.Reliability_RELIABILITY_RELIABLE_REPLAY,
			Payload:        &pb.TelemetryEvent_State{State: &pb.State{Cpu: 25, MemUsed: 1024, Uptime: 3600}},
		})
	}
	batches := make([]*pb.ReplicationBatch, 0, collectors)
	for collectorIndex := 0; collectorIndex < collectors; collectorIndex++ {
		observerID := fmt.Sprintf("collector-%02d", collectorIndex)
		observations := make([]*pb.TelemetryObservation, 0, agents)
		for _, event := range events {
			observations = append(observations, &pb.TelemetryObservation{
				EventId: event.GetEventId(), ObserverId: observerID, ReceivedAtUnixNano: now.UnixNano(),
			})
		}
		batches = append(batches, &pb.ReplicationBatch{
			CollectorUuid: observerID, ReplicationSession: benchmarkID(uint64(iteration+1), uint64(collectorIndex+1)),
			BatchSequence: 1, SpoolThroughId: uint64(agents * 2), Events: events, Observations: observations,
		})
	}
	return batches
}

func benchmarkID(high, low uint64) []byte {
	id := make([]byte, 16)
	binary.BigEndian.PutUint64(id[:8], high)
	binary.BigEndian.PutUint64(id[8:], low)
	return id
}

func benchmarkDimension(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
