package telemetry

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"google.golang.org/protobuf/proto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRetentionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.TelemetryEvent{}, &model.TelemetryObservation{}, &model.TelemetryGap{},
		&model.StateRollup{}, &model.AvailabilityBucket{}, &model.AvailabilityIncident{},
		&model.IncidentRevision{}, &model.ConnectionLatencyBucket{},
		&model.ObserverPathBucket{}, &model.ObserverHealthBucket{},
		&model.CollectorReplicationReceipt{}, &model.TelemetryAlert{}, &model.TelemetryDataLoss{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestDrainRetentionDeletesMoreThanOneBatch(t *testing.T) {
	db := newRetentionDB(t)
	now := time.Now()
	old := now.Add(-40 * 24 * time.Hour).UnixNano()
	node := bytes.Repeat([]byte{9}, 16)
	rows := make([]model.TelemetryObservation, 1500)
	for i := range rows {
		eventID := make([]byte, 16)
		eventID[0] = byte(i)
		eventID[1] = byte(i >> 8)
		rows[i] = model.TelemetryObservation{
			EventID: eventID, ObserverID: "primary", NodeUUID: node, ReceivedAt: old,
		}
	}
	if err := db.CreateInBatches(rows, 500).Error; err != nil {
		t.Fatal(err)
	}
	deleted, err := DrainRetention(context.Background(), db, RetentionPolicy{BatchSize: 100, MaxRuntime: time.Minute}, now)
	if err != nil {
		t.Fatal(err)
	}
	if deleted < 1500 {
		t.Fatalf("deleted=%d", deleted)
	}
	var count int64
	if err := db.Model(&model.TelemetryObservation{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("remaining=%d", count)
	}
}

func TestDrainRetentionKeepsFreshRowsAndStripsOldStatePayload(t *testing.T) {
	db := newRetentionDB(t)
	now := time.Now()
	node, session := bytes.Repeat([]byte{1}, 16), bytes.Repeat([]byte{2}, 16)
	oldAt := now.Add(-8 * time.Hour)
	freshAt := now.Add(-time.Minute)
	oldID, _ := EventID(node, session, 1)
	freshID, _ := EventID(node, session, 2)
	event := &pb.TelemetryEvent{EventId: oldID, NodeUuid: node, SessionId: session, Sequence: 1, EventType: pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_STATE, Payload: &pb.TelemetryEvent_State{State: &pb.State{Cpu: 10}}}
	encoded, _ := proto.Marshal(event)
	if err := db.Create(&model.TelemetryEvent{
		EventID: oldID, NodeUUID: node, SessionID: session, Sequence: 1,
		EventType: int32(pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_STATE), CollectedAt: oldAt.UnixNano(),
		Payload: encoded, PayloadRetained: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TelemetryEvent{
		EventID: freshID, NodeUUID: node, SessionID: session, Sequence: 2,
		EventType: int32(pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_STATE), CollectedAt: freshAt.UnixNano(),
		Payload: encoded, PayloadRetained: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TelemetryObservation{EventID: freshID, ObserverID: "primary", NodeUUID: node, ReceivedAt: freshAt.UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	staleBucket := now.Add(-40 * 24 * time.Hour).UnixNano()
	if err := db.Create(&model.ObserverPathBucket{NodeUUID: node, ObserverID: "primary", BucketStart: staleBucket}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ObserverPathBucket{NodeUUID: node, ObserverID: "primary", BucketStart: freshAt.UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ObserverHealthBucket{ObserverID: "primary", BucketStart: staleBucket}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ObserverHealthBucket{ObserverID: "primary", BucketStart: freshAt.UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CollectorReplicationReceipt{
		CollectorUUID: "c1", ReplicationSession: bytes.Repeat([]byte{3}, 16), BatchSequence: 1, CreatedAt: now.Add(-10 * 24 * time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CollectorReplicationReceipt{
		CollectorUUID: "c1", ReplicationSession: bytes.Repeat([]byte{4}, 16), BatchSequence: 2, CreatedAt: now.Add(-time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := DrainRetention(context.Background(), db, RetentionPolicy{BatchSize: 50, MaxRuntime: time.Minute}, now); err != nil {
		t.Fatal(err)
	}
	var oldEvent, freshEvent model.TelemetryEvent
	if err := db.First(&oldEvent, "event_id = ?", oldID).Error; err != nil {
		t.Fatal(err)
	}
	if oldEvent.PayloadRetained || len(oldEvent.Payload) != 0 {
		t.Fatalf("old payload still retained: %#v", oldEvent)
	}
	if err := db.First(&freshEvent, "event_id = ?", freshID).Error; err != nil {
		t.Fatal(err)
	}
	if !freshEvent.PayloadRetained {
		t.Fatal("fresh payload stripped")
	}
	var paths, health, receipts, observations int64
	if err := db.Model(&model.ObserverPathBucket{}).Count(&paths).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.ObserverHealthBucket{}).Count(&health).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.CollectorReplicationReceipt{}).Count(&receipts).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.TelemetryObservation{}).Count(&observations).Error; err != nil {
		t.Fatal(err)
	}
	if paths != 1 || health != 1 || receipts != 1 || observations != 1 {
		t.Fatalf("paths=%d health=%d receipts=%d observations=%d", paths, health, receipts, observations)
	}
}

func TestPolicyFromConfigUsesDocumentedDefaults(t *testing.T) {
	policy := PolicyFromConfig(model.RetentionConfig{})
	if policy.BatchSize != DefaultRetentionBatch || policy.MaxRuntime != DefaultRetentionBudget {
		t.Fatalf("batch=%d runtime=%s", policy.BatchSize, policy.MaxRuntime)
	}
	if policy.Receipt != DefaultReceiptRetain || policy.CompactMinBytes != DefaultCompactMinBytes || !policy.AutoCompact {
		t.Fatalf("receipt=%s compact=%d auto=%v", policy.Receipt, policy.CompactMinBytes, policy.AutoCompact)
	}
	off := false
	if PolicyFromConfig(model.RetentionConfig{AutoCompact: &off}).AutoCompact {
		t.Fatal("explicit auto_compact=false must stick")
	}
}

func TestPayloadRetentionSkipsCurrentMinute(t *testing.T) {
	db := newRetentionDB(t)
	now := time.Now().Truncate(time.Minute).Add(30 * time.Second)
	node, session := bytes.Repeat([]byte{5}, 16), bytes.Repeat([]byte{6}, 16)
	eventID, _ := EventID(node, session, 1)
	event := &pb.TelemetryEvent{EventId: eventID, Payload: &pb.TelemetryEvent_State{State: &pb.State{Cpu: 1}}}
	encoded, _ := proto.Marshal(event)
	if err := db.Create(&model.TelemetryEvent{
		EventID: eventID, NodeUUID: node, SessionID: session, Sequence: 1,
		EventType:   int32(pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_STATE),
		CollectedAt: now.UnixNano(), Payload: encoded, PayloadRetained: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := DrainRetention(context.Background(), db, RetentionPolicy{StateRaw: time.Nanosecond, MaxRuntime: time.Minute}, now); err != nil {
		t.Fatal(err)
	}
	var row model.TelemetryEvent
	if err := db.First(&row, "event_id = ?", eventID).Error; err != nil {
		t.Fatal(err)
	}
	if !row.PayloadRetained {
		t.Fatal("current-minute payload should stay")
	}
}
