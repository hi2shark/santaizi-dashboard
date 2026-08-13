package collector

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"github.com/hi2shark/santaizi-dashboard/service/telemetry"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAgentCollectorPrimaryPipelineSurvivesLostReplicationAck(t *testing.T) {
	collectorStore := openTestStore(t)
	primaryDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeGormDB(primaryDB) })
	if err := primaryDB.AutoMigrate(
		&model.TelemetryEvent{}, &model.TelemetryObservation{}, &model.TelemetryGap{},
		&model.ObserverHealthBucket{}, &model.ObserverPathBucket{}, &model.ObserverAssignment{},
		&model.AvailabilityRecomputeQueue{}, &model.CollectorReplicationReceipt{},
		&model.CollectorRuntime{}, &model.TelemetryDataLoss{},
	); err != nil {
		t.Fatal(err)
	}
	primaryStore := telemetry.NewStore(primaryDB)
	node, session := bytes.Repeat([]byte{0x51}, 16), bytes.Repeat([]byte{0x52}, 16)
	now := time.Now()
	fact := collectorEvent(t, node, session, 1)
	result, err := collectorStore.Ingest(context.Background(), &pb.TelemetryBatch{Records: []*pb.TelemetryRecord{{
		Record: &pb.TelemetryRecord_Event{Event: fact},
	}}}, "collector-pipeline", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Acks) != 1 || result.Acks[0].GetAckThrough() != 1 {
		t.Fatalf("agent ACK=%#v", result.Acks)
	}
	outbox, err := collectorStore.ReadOutbox(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	batch := &pb.ReplicationBatch{
		CollectorUuid: "collector-pipeline", ReplicationSession: bytes.Repeat([]byte{0x53}, 16),
		BatchSequence: 1, SpoolThroughId: outbox.Through, Events: outbox.Events, Observations: outbox.Observations,
	}
	// Retry the same identity to model a committed transaction whose ACK was lost.
	for attempt := 0; attempt < 2; attempt++ {
		committed, err := primaryStore.Replicate(context.Background(), batch, now.Add(time.Duration(attempt)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if committed != outbox.Through {
			t.Fatalf("primary committed=%d want=%d", committed, outbox.Through)
		}
	}
	if err := collectorStore.CommitReplicationAck(context.Background(), outbox.Through); err != nil {
		t.Fatal(err)
	}
	var events, observations, pending int64
	primaryDB.Model(&model.TelemetryEvent{}).Count(&events)
	primaryDB.Model(&model.TelemetryObservation{}).Count(&observations)
	collectorStore.db.Model(&model.CollectorOutbox{}).Count(&pending)
	if events != 1 || observations != 1 || pending != 0 {
		t.Fatalf("events=%d observations=%d pending=%d", events, observations, pending)
	}
}
