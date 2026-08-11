package singleton

import (
	"bytes"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"google.golang.org/protobuf/proto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestV2NodeReplacementCreatesIdentityLifecycleFact(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Server{}, &model.ServerNodeBinding{}, &model.ObserverAssignment{}, &model.Collector{}, &model.CollectorScope{},
		&model.TelemetryEvent{}, &model.TelemetryObservation{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Server{Common: model.Common{ID: 7}, Name: "node-7", Secret: "secret-7"}).Error; err != nil {
		t.Fatal(err)
	}
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })

	oldNode := bytes.Repeat([]byte{0x11}, 16)
	newNode := bytes.Repeat([]byte{0x22}, 16)
	now := time.Unix(1_800_000_000, 0)
	if _, err := BindServerNodeForProtocol(7, oldNode, now, pb.SourceProtocol_SOURCE_PROTOCOL_SANTAIZI_V2); err != nil {
		t.Fatal(err)
	}
	if changed, err := BindServerNodeForProtocol(7, newNode, now.Add(time.Minute), pb.SourceProtocol_SOURCE_PROTOCOL_SANTAIZI_V2); err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}

	var row model.TelemetryEvent
	if err := db.Where("node_uuid = ? AND event_type = ?", newNode, pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_LIFECYCLE).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	var event pb.TelemetryEvent
	if err := proto.Unmarshal(row.Payload, &event); err != nil {
		t.Fatal(err)
	}
	if event.GetPriority() != pb.TelemetryPriority_TELEMETRY_PRIORITY_P0_CRITICAL ||
		event.GetLifecycle().GetKind() != pb.LifecycleKind_LIFECYCLE_KIND_AGENT_IDENTITY_CHANGED ||
		!bytes.Equal(event.GetLifecycle().GetPreviousNodeUuid(), oldNode) {
		t.Fatalf("unexpected lifecycle fact: %#v", &event)
	}
	var observation model.TelemetryObservation
	if err := db.First(&observation, "event_id = ? AND observer_id = ?", row.EventID, primaryObserverID).Error; err != nil {
		t.Fatal(err)
	}
	var oldBinding model.ServerNodeBinding
	if err := db.First(&oldBinding, "server_id = ? AND node_uuid = ?", 7, oldNode).Error; err != nil {
		t.Fatal(err)
	}
	if oldBinding.Current || oldBinding.ValidTo == 0 {
		t.Fatalf("old binding not closed: %#v", oldBinding)
	}
}

func TestHistoricalReplayDoesNotOverwriteFreshRuntime(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Server{}, &model.ServerNodeBinding{}, &model.ObserverAssignment{}, &model.Collector{}, &model.CollectorScope{},
		&model.ServerRuntime{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Server{Common: model.Common{ID: 9}, Name: "node-9", Secret: "secret-9"}).Error; err != nil {
		t.Fatal(err)
	}
	previousDB, previousConf, previousServers := DB, Conf, ServerList
	DB = db
	Conf = &model.Config{Telemetry: model.TelemetryConfig{OfflineThresholdSeconds: 30}}
	ServerList = map[uint64]*model.Server{9: {Common: model.Common{ID: 9}, State: &model.HostState{}, Host: &model.Host{}}}
	t.Cleanup(func() {
		DB, Conf, ServerList = previousDB, previousConf, previousServers
	})

	node := bytes.Repeat([]byte{0x31}, 16)
	session := bytes.Repeat([]byte{0x32}, 16)
	now := time.Now()
	if _, err := BindServerNode(9, node, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ApplyRealtimeSnapshot(&pb.RealtimeSnapshot{
		NodeUuid: node, SessionId: session, LatestSequence: 10,
		CollectedAtUnixNano: now.UnixNano(), State: &pb.State{Cpu: 90},
	}, now); err != nil {
		t.Fatal(err)
	}
	oldEventID := make([]byte, 16)
	if err := ApplyV2Event(&pb.TelemetryEvent{
		EventId: oldEventID, NodeUuid: node, SessionId: session, Sequence: 5,
		CollectedAtUnixNano: now.Add(-time.Hour).UnixNano(),
		Payload:             &pb.TelemetryEvent_State{State: &pb.State{Cpu: 10}},
	}, now); err != nil {
		t.Fatal(err)
	}
	var runtime model.ServerRuntime
	if err := db.First(&runtime, "server_id = ?", 9).Error; err != nil {
		t.Fatal(err)
	}
	var state pb.State
	if err := proto.Unmarshal(runtime.StatePayload, &state); err != nil {
		t.Fatal(err)
	}
	if runtime.CurrentSequence != 10 || state.GetCpu() != 90 || ServerList[9].State.CPU != 90 {
		t.Fatalf("historical replay overwrote runtime: sequence=%d state=%v memory=%v", runtime.CurrentSequence, state.GetCpu(), ServerList[9].State.CPU)
	}
}

func TestBindingAndTagChangeRefreshCollectorAssignments(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Server{}, &model.ServerNodeBinding{}, &model.ObserverAssignment{},
		&model.Collector{}, &model.CollectorScope{}, &model.TelemetryEvent{}, &model.TelemetryObservation{},
	); err != nil {
		t.Fatal(err)
	}
	server := model.Server{Common: model.Common{ID: 12}, Name: "node-12", Secret: "secret-12", Tag: "edge"}
	collector := model.Collector{CollectorUUID: "collector-edge", Name: "Edge", Address: "edge:5555", TokenHash: bytes.Repeat([]byte{1}, 32), RegistrationToken: "test-registration-token", Generation: 3, ConfigVersion: 1}
	if err := db.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&collector).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CollectorScope{CollectorUUID: collector.CollectorUUID, ScopeType: "tag", ScopeValue: "edge"}).Error; err != nil {
		t.Fatal(err)
	}
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })

	node := bytes.Repeat([]byte{0x41}, 16)
	now := time.Unix(1_800_100_000, 0)
	if _, err := BindServerNodeForProtocol(server.ID, node, now, pb.SourceProtocol_SOURCE_PROTOCOL_SANTAIZI_V2); err != nil {
		t.Fatal(err)
	}
	var assignment model.ObserverAssignment
	if err := db.First(&assignment, "node_uuid = ? AND observer_id = ? AND valid_to = 0", node, collector.CollectorUUID).Error; err != nil {
		t.Fatal(err)
	}
	if assignment.Generation != 3 || assignment.ConfigVersion != 2 {
		t.Fatalf("assignment=%#v", assignment)
	}

	if err := db.Model(&server).Update("tag", "core").Error; err != nil {
		t.Fatal(err)
	}
	if err := RefreshObserverAssignmentsForServer(server.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&assignment, assignment.ID).Error; err != nil {
		t.Fatal(err)
	}
	if assignment.ValidTo == 0 {
		t.Fatal("scope removal deleted or left the historical assignment active")
	}
	if err := db.First(&collector, "collector_uuid = ?", collector.CollectorUUID).Error; err != nil {
		t.Fatal(err)
	}
	if collector.ConfigVersion != 3 {
		t.Fatalf("collector config version=%d", collector.ConfigVersion)
	}
}
