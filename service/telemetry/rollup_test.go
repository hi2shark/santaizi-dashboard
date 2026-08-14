package telemetry

import (
	"bytes"
	"context"
	"math"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"google.golang.org/protobuf/proto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCounterDeltasHandleRestartWrapAndAnomalyWithoutNegativeTraffic(t *testing.T) {
	states := []*pb.State{
		{Uptime: 100, NetInTransfer: 1000, NetOutTransfer: math.MaxUint64 - 10},
		{Uptime: 105, NetInTransfer: 1200, NetOutTransfer: 20},
		{Uptime: 2, NetInTransfer: 100, NetOutTransfer: 50},
		{Uptime: 3, NetInTransfer: 90, NetOutTransfer: 60},
		{Uptime: 4, NetInTransfer: 1 << 60, NetOutTransfer: 1 << 60},
	}
	inbound, outbound := counterDeltas(states)
	if inbound != 200 {
		t.Fatalf("inbound=%d", inbound)
	}
	if outbound != 41 {
		t.Fatalf("outbound=%d", outbound)
	}
}

func TestRawRollupAndPayloadRetentionRequireCompletedMinute(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TelemetryEvent{}, &model.StateRollup{}, &model.TelemetryObservation{}, &model.TelemetryGap{}, &model.AvailabilityBucket{}, &model.AvailabilityIncident{}, &model.ConnectionLatencyBucket{}); err != nil {
		t.Fatal(err)
	}
	node, session := bytes.Repeat([]byte{1}, 16), bytes.Repeat([]byte{2}, 16)
	end := time.Now().Add(-8 * time.Hour).Truncate(time.Minute)
	start := end.Add(-time.Minute)
	for index, state := range []*pb.State{
		{Cpu: 10, MemUsed: 100, Uptime: 100, NetInTransfer: 1000, NetInSpeed: 100, NetOutSpeed: 40},
		{Cpu: 30, MemUsed: 300, Uptime: 105, NetInTransfer: 1250, NetInSpeed: 300, NetOutSpeed: 80},
	} {
		sequence := uint64(index + 1)
		eventID, _ := EventID(node, session, sequence)
		event := &pb.TelemetryEvent{
			EventId: eventID, NodeUuid: node, SessionId: session, Sequence: sequence,
			EventType:           pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_STATE,
			Priority:            pb.TelemetryPriority_TELEMETRY_PRIORITY_P2_NORMAL,
			CollectedAtUnixNano: start.Add(time.Duration(index+1) * 10 * time.Second).UnixNano(),
			Payload:             &pb.TelemetryEvent_State{State: state},
		}
		encoded, _ := proto.Marshal(event)
		if err := db.Create(&model.TelemetryEvent{
			EventID: eventID, NodeUUID: node, SessionID: session, Sequence: sequence,
			EventType: int32(event.GetEventType()), Priority: int32(event.GetPriority()),
			CollectedAt: event.GetCollectedAtUnixNano(), Payload: encoded, PayloadRetained: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	worker := NewRollupWorker(db, RetentionPolicy{StateRaw: 6 * time.Hour, BatchSize: 100})
	if err := worker.rollupRawWindow(context.Background(), start, end); err != nil {
		t.Fatal(err)
	}
	var row model.StateRollup
	if err := db.First(&row, "node_uuid = ? AND resolution = ? AND window_start = ?", node, "1m", start.UnixNano()).Error; err != nil {
		t.Fatal(err)
	}
	payload := new(pb.StateRollupPayload)
	if err := proto.Unmarshal(row.Payload, payload); err != nil {
		t.Fatal(err)
	}
	if payload.GetSampleCount() != 2 || payload.GetAverage().GetCpu() != 20 || payload.GetNetInTotal() != 250 {
		t.Fatalf("payload=%#v", payload)
	}
	if payload.GetAverage().GetNetInSpeed() != 200 || payload.GetAverage().GetNetOutSpeed() != 60 {
		t.Fatalf("speed average in=%d out=%d", payload.GetAverage().GetNetInSpeed(), payload.GetAverage().GetNetOutSpeed())
	}
	if err := worker.ApplyRetention(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	var retained int64
	if err := db.Model(&model.TelemetryEvent{}).Where("payload_retained = ?", true).Count(&retained).Error; err != nil {
		t.Fatal(err)
	}
	if retained != 0 {
		t.Fatalf("retained state payloads=%d", retained)
	}
}

func TestHourlyRollupPreservesMinuteExtremaAndWeightedAverage(t *testing.T) {
	start := time.Unix(1_800_000_000, 0).Truncate(time.Hour)
	rows := make([]model.StateRollup, 0, 2)
	for index, payload := range []*pb.StateRollupPayload{
		{SampleCount: 1, Minimum: &pb.State{Cpu: 5, MemUsed: 100}, Average: &pb.State{Cpu: 10, MemUsed: 200}, Maximum: &pb.State{Cpu: 20, MemUsed: 300}, NetInTotal: 40},
		{SampleCount: 3, Minimum: &pb.State{Cpu: 30, MemUsed: 400}, Average: &pb.State{Cpu: 50, MemUsed: 600}, Maximum: &pb.State{Cpu: 90, MemUsed: 900}, NetInTotal: 60},
	} {
		encoded, err := proto.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, model.StateRollup{WindowStart: start.Add(time.Duration(index) * time.Minute).UnixNano(), Payload: encoded, SampleCount: payload.GetSampleCount()})
	}
	result := aggregateRollups(rows, start, start.Add(time.Hour))
	if result.GetSampleCount() != 4 || result.GetMinimum().GetCpu() != 5 || result.GetMaximum().GetCpu() != 90 || result.GetAverage().GetCpu() != 40 || result.GetNetInTotal() != 100 {
		t.Fatalf("hourly rollup=%#v", result)
	}
}
