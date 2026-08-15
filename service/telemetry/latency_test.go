package telemetry

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRecordLatencySampleDedupesAndAggregatesMinute(t *testing.T) {
	db := newConnectionDB(t)
	node := bytes.Repeat([]byte{0x11}, 16)
	first := time.Unix(1_700_000_010, 0)
	if err := RecordLatencySample(db, LatencySample{
		Kind: LatencyKindPath, NodeUUID: node, ObserverID: PrimaryObserverID, RttMs: 10, SampledAt: first.UnixNano(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := RecordLatencySample(db, LatencySample{
		Kind: LatencyKindPath, NodeUUID: node, ObserverID: PrimaryObserverID, RttMs: 99, SampledAt: first.UnixNano(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := RecordLatencySample(db, LatencySample{
		Kind: LatencyKindPath, NodeUUID: node, ObserverID: PrimaryObserverID, RttMs: 30, SampledAt: first.Add(20 * time.Second).UnixNano(),
	}); err != nil {
		t.Fatal(err)
	}
	rows, total, err := ListConnectionLatency(db, LatencyFilter{Kind: LatencyKindPath, ObserverID: PrimaryObserverID}, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Count != 2 || rows[0].MinMs != 10 || rows[0].MaxMs != 30 || rows[0].AvgMs != 20 {
		t.Fatalf("bucket=%#v total=%d", rows, total)
	}
}

func TestRecordAgentSinkLatencyWritesPathBucket(t *testing.T) {
	db := newConnectionDB(t)
	node := bytes.Repeat([]byte{0x22}, 16)
	sampled := time.Unix(1_700_000_000, 0).UnixNano()
	if err := RecordAgentSinkLatency(db, node, &pb.AgentRuntime{Sinks: []*pb.SinkRuntime{
		{EndpointId: PrimaryObserverID, Connected: true, LastRttMs: 7.5, RttSampledAtUnixNano: sampled},
	}}); err != nil {
		t.Fatal(err)
	}
	rows, total, err := ListConnectionLatency(db, LatencyFilter{Kind: LatencyKindPath}, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || rows[0].AvgMs != 7.5 || rows[0].ObserverID != PrimaryObserverID {
		t.Fatalf("rows=%#v total=%d", rows, total)
	}
}

func TestRecordAgentSinkLatencySkipsUnhandshaked(t *testing.T) {
	db := newConnectionDB(t)
	now := time.Unix(1_700_000_090, 0)
	node := bytes.Repeat([]byte{0x33}, 16)
	sampled := now.UnixNano()
	createCollector(t, db, "collector-stale", "Stale")
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "collector-stale", Status: "online", LastSeen: now.Add(-2 * time.Minute).UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := recordAgentSinkLatency(db, node, &pb.AgentRuntime{Sinks: []*pb.SinkRuntime{
		{EndpointId: PrimaryObserverID, Connected: false, LastRttMs: 7.5, RttSampledAtUnixNano: sampled},
		{EndpointId: "collector-stale", Connected: true, LastRttMs: 40, RttSampledAtUnixNano: sampled},
	}}, now); err != nil {
		t.Fatal(err)
	}
	_, total, err := ListConnectionLatency(db, LatencyFilter{Kind: LatencyKindPath}, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("unhandshaked sinks should not write buckets, total=%d", total)
	}
}

func TestApplyRetentionDeletesOldConnectionLatency(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TelemetryEvent{}, &model.StateRollup{}, &model.TelemetryObservation{}, &model.TelemetryGap{}, &model.AvailabilityBucket{}, &model.AvailabilityIncident{}, &model.ConnectionLatencyBucket{}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour).UnixNano()
	fresh := time.Now().Add(-time.Hour).UnixNano()
	if err := db.Create(&model.ConnectionLatencyBucket{Kind: LatencyKindCollectorHeartbeat, CollectorUUID: "c1", NodeUUID: latencyNodeKey(nil), BucketStart: old, MinMs: 1, MaxMs: 1, SumMs: 1, Count: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ConnectionLatencyBucket{Kind: LatencyKindCollectorHeartbeat, CollectorUUID: "c1", NodeUUID: latencyNodeKey(nil), BucketStart: fresh, MinMs: 2, MaxMs: 2, SumMs: 2, Count: 1}).Error; err != nil {
		t.Fatal(err)
	}
	worker := NewRollupWorker(db, RetentionPolicy{BatchSize: 100})
	if err := worker.ApplyRetention(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.ConnectionLatencyBucket{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("retained=%d", count)
	}
}
