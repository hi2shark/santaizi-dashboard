package telemetry

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"google.golang.org/protobuf/proto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCollectorStatusTimeout(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	if got := CollectorStatus(0, now); got != CollectorStatusUnknown {
		t.Fatalf("empty last seen: %s", got)
	}
	if got := CollectorStatus(now.Add(-CollectorTimeout+time.Second).UnixNano(), now); got != CollectorStatusOnline {
		t.Fatalf("fresh heartbeat: %s", got)
	}
	if got := CollectorStatus(now.Add(-CollectorTimeout-time.Second).UnixNano(), now); got != CollectorStatusOffline {
		t.Fatalf("stale heartbeat: %s", got)
	}
}

func newConnectionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Server{}, &model.ServerNodeBinding{}, &model.Collector{}, &model.CollectorRuntime{},
		&model.ObserverAssignment{}, &model.ObserverPathBucket{}, &model.AgentTelemetryRuntime{},
		&model.ConnectionLatencyBucket{}, &model.ConnectionLatencyCursor{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestLoadConnectionSummaryCountsCollectorHeartbeat(t *testing.T) {
	db := newConnectionDB(t)
	now := time.Unix(1_700_000_090, 0)
	createCollector(t, db, "online-c", "Online")
	createCollector(t, db, "offline-c", "Offline")
	createCollector(t, db, "unknown-c", "Unknown")
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "online-c", Status: "online", LastSeen: now.Add(-10 * time.Second).UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "offline-c", Status: "online", LastSeen: now.Add(-2 * time.Minute).UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	summary, err := LoadConnectionSummary(db, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.CollectorsTotal != 3 || summary.CollectorsOnline != 1 || summary.CollectorsOffline != 1 || summary.CollectorsUnknown != 1 {
		t.Fatalf("summary=%#v", summary)
	}
}

func TestLoadConnectionPathsJoinsAssignmentPathAndSink(t *testing.T) {
	db := newConnectionDB(t)
	now := time.Unix(1_700_000_000, 0)
	node := bytes.Repeat([]byte{9}, 16)
	server := model.Server{Name: "edge-a", Secret: "secret"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ServerNodeBinding{ServerID: server.ID, NodeUUID: node, Current: true, Reason: "test", ValidFrom: now.UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	createCollector(t, db, "collector-east", "East")
	for _, observer := range []string{PrimaryObserverID, "collector-east"} {
		if err := db.Create(&model.ObserverAssignment{NodeUUID: node, ObserverID: observer, ValidFrom: now.UnixNano(), ConfigVersion: 1, Generation: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.ObserverPathBucket{NodeUUID: node, ObserverID: PrimaryObserverID, BucketStart: now.UnixNano(), Seen: true, LastSeenAt: now.UnixNano(), Revision: 1}).Error; err != nil {
		t.Fatal(err)
	}
	runtime, err := proto.Marshal(&pb.AgentRuntime{Sinks: []*pb.SinkRuntime{
		{EndpointId: PrimaryObserverID, Connected: true, PendingEvents: 2, AckThrough: 11, LastRttMs: 12.5, RttSampledAtUnixNano: now.UnixNano()},
		{EndpointId: "collector-east", Connected: false, LastError: "dial timeout", PendingEvents: 4},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AgentTelemetryRuntime{NodeUUID: node, SinkCursors: runtime}).Error; err != nil {
		t.Fatal(err)
	}

	paths, err := LoadConnectionPaths(db, PathFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("paths=%d", len(paths))
	}
	byObserver := map[string]ConnectionPath{}
	for _, path := range paths {
		byObserver[path.ObserverID] = path
	}
	primary := byObserver[PrimaryObserverID]
	if primary.ServerID != server.ID || primary.ServerName != "edge-a" || primary.ObserverKind != ObserverKindPrimary || !primary.Sink.Connected || primary.LastSeen != now.UnixNano() || primary.NodeUUID != hex.EncodeToString(node) || primary.Sink.LastRttMs != 12.5 {
		t.Fatalf("primary=%#v", primary)
	}
	east := byObserver["collector-east"]
	if east.ObserverKind != ObserverKindCollector || east.ObserverName != "East" || east.Sink.Connected || east.Sink.LastError != "dial timeout" || east.LastSeen != 0 {
		t.Fatalf("east=%#v", east)
	}

	filtered, err := LoadConnectionPaths(db, PathFilter{ServerID: server.ID, ObserverID: PrimaryObserverID})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].ObserverID != PrimaryObserverID {
		t.Fatalf("filtered=%#v", filtered)
	}
}

func TestLoadConnectionPathsClearsOfflineCollectorRTT(t *testing.T) {
	db := newConnectionDB(t)
	now := time.Unix(1_700_000_090, 0)
	node := bytes.Repeat([]byte{9}, 16)
	server := model.Server{Name: "edge-a", Secret: "secret"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ServerNodeBinding{ServerID: server.ID, NodeUUID: node, Current: true, Reason: "test", ValidFrom: now.UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	createCollector(t, db, "collector-live", "Live")
	createCollector(t, db, "collector-stale", "Stale")
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "collector-live", Status: "online", LastSeen: now.Add(-10 * time.Second).UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "collector-stale", Status: "online", LastSeen: now.Add(-2 * time.Minute).UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	for _, observer := range []string{PrimaryObserverID, "collector-live", "collector-stale"} {
		if err := db.Create(&model.ObserverAssignment{NodeUUID: node, ObserverID: observer, ValidFrom: now.UnixNano(), ConfigVersion: 1, Generation: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	runtime, err := proto.Marshal(&pb.AgentRuntime{Sinks: []*pb.SinkRuntime{
		{EndpointId: PrimaryObserverID, Connected: true, LastRttMs: 12.5, RttSampledAtUnixNano: now.UnixNano()},
		{EndpointId: "collector-live", Connected: true, LastRttMs: 33, RttSampledAtUnixNano: now.UnixNano()},
		{EndpointId: "collector-stale", Connected: true, LastRttMs: 99, RttSampledAtUnixNano: now.UnixNano(), LastError: "old"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AgentTelemetryRuntime{NodeUUID: node, SinkCursors: runtime}).Error; err != nil {
		t.Fatal(err)
	}

	paths, err := loadConnectionPaths(db, PathFilter{}, now)
	if err != nil {
		t.Fatal(err)
	}
	byObserver := map[string]ConnectionPath{}
	for _, path := range paths {
		byObserver[path.ObserverID] = path
	}
	if !byObserver[PrimaryObserverID].Sink.Connected || byObserver[PrimaryObserverID].Sink.LastRttMs != 12.5 {
		t.Fatalf("primary=%#v", byObserver[PrimaryObserverID])
	}
	if !byObserver["collector-live"].Sink.Connected || byObserver["collector-live"].Sink.LastRttMs != 33 {
		t.Fatalf("live=%#v", byObserver["collector-live"])
	}
	stale := byObserver["collector-stale"]
	if stale.Sink.Connected || stale.Sink.LastRttMs != 0 || stale.Sink.RttSampledAt != 0 || stale.Sink.LastError != "old" {
		t.Fatalf("stale should drop RTT and stay disconnected: %#v", stale)
	}
}

func TestUpsertCollectorRuntimePreservesLastSyncOnHeartbeat(t *testing.T) {
	db := newConnectionDB(t)
	now := time.Unix(1_700_000_000, 0)
	runtime := &pb.CollectorRuntime{
		SpoolSize: 8, PendingRecords: 1, LastPrimarySeenUnixNano: now.UnixNano(), ConnectedAgents: 3,
		HeartbeatRttMs: 18.5, HeartbeatRttSampledAtUnixNano: now.UnixNano(),
	}
	if err := UpsertCollectorRuntime(db, CollectorRuntimeFromProto("c1", runtime, now, true), true); err != nil {
		t.Fatal(err)
	}
	later := now.Add(15 * time.Second)
	if err := UpsertCollectorRuntime(db, CollectorRuntimeFromProto("c1", runtime, later, false), false); err != nil {
		t.Fatal(err)
	}
	var row model.CollectorRuntime
	if err := db.First(&row, "collector_uuid = ?", "c1").Error; err != nil {
		t.Fatal(err)
	}
	if row.LastSeen != later.UnixNano() || row.LastSync != now.UnixNano() || row.LastPrimarySeen != now.UnixNano() || row.ConnectedAgents != 3 || row.HeartbeatRttMs != 18.5 {
		t.Fatalf("row=%#v", row)
	}
	buckets, total, err := ListConnectionLatency(db, LatencyFilter{Kind: LatencyKindCollectorHeartbeat, CollectorUUID: "c1"}, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(buckets) != 1 || buckets[0].AvgMs != 18.5 || buckets[0].Count != 1 {
		t.Fatalf("latency=%#v total=%d", buckets, total)
	}
}

func TestUpsertCollectorRuntimePreservesSoftwareVersionOnEmpty(t *testing.T) {
	db := newConnectionDB(t)
	now := time.Unix(1_700_000_000, 0)
	first := &pb.CollectorRuntime{SoftwareVersion: "1.2.3", ConnectedAgents: 1}
	if err := UpsertCollectorRuntime(db, CollectorRuntimeFromProto("c1", first, now, true), true); err != nil {
		t.Fatal(err)
	}
	later := now.Add(15 * time.Second)
	empty := &pb.CollectorRuntime{ConnectedAgents: 2}
	if err := UpsertCollectorRuntime(db, CollectorRuntimeFromProto("c1", empty, later, false), false); err != nil {
		t.Fatal(err)
	}
	var row model.CollectorRuntime
	if err := db.First(&row, "collector_uuid = ?", "c1").Error; err != nil {
		t.Fatal(err)
	}
	if row.SoftwareVersion != "1.2.3" || row.ConnectedAgents != 2 {
		t.Fatalf("empty update should keep software version, row=%#v", row)
	}
	upgrade := &pb.CollectorRuntime{SoftwareVersion: "1.2.4", ConnectedAgents: 3}
	if err := UpsertCollectorRuntime(db, CollectorRuntimeFromProto("c1", upgrade, later.Add(time.Second), false), false); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&row, "collector_uuid = ?", "c1").Error; err != nil {
		t.Fatal(err)
	}
	if row.SoftwareVersion != "1.2.4" {
		t.Fatalf("non-empty update should replace software version, row=%#v", row)
	}
}

func createCollector(t *testing.T, db *gorm.DB, id, name string) {
	t.Helper()
	if err := db.Create(&model.Collector{
		CollectorUUID: id, Name: name, Address: id + ":5556", TokenHash: bytes.Repeat([]byte{3}, 32), RegistrationToken: "token-" + id,
	}).Error; err != nil {
		t.Fatal(err)
	}
}
