package telemetry

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAlertWorkerPersistsDefaultMutedConnectivityAndDataLoss(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AvailabilityIncident{}, &model.Collector{}, &model.CollectorRuntime{}, &model.TelemetryDataLoss{}, &model.TelemetryEvent{}, &model.TelemetryAlert{}, &model.Server{}, &model.ServerNodeBinding{}); err != nil {
		t.Fatal(err)
	}
	node := bytes.Repeat([]byte{1}, 16)
	if err := db.Create(&model.AvailabilityIncident{
		NodeUUID: node, InitialClassification: AlertConnectivityDegraded, CurrentClassification: AlertConnectivityDegraded,
		Revision: 1, StartedAt: time.Now().UnixNano(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TelemetryDataLoss{
		FactID: bytes.Repeat([]byte{2}, 16), ComponentID: "collector-a", OccurredAt: time.Now().UnixNano(), LostRecords: 3,
	}).Error; err != nil {
		t.Fatal(err)
	}
	notifications := 0
	worker := NewAlertWorker(db, AlertPolicy{}, func(string, string) { notifications++ })
	if err := worker.Scan(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	var alerts []model.TelemetryAlert
	if err := db.Order("alert_type ASC").Find(&alerts).Error; err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 2 || notifications != 0 {
		t.Fatalf("alerts=%d notifications=%d", len(alerts), notifications)
	}
	for _, alert := range alerts {
		if alert.Notified {
			t.Fatalf("default-muted alert was marked notified: %#v", alert)
		}
	}
}

func TestAlertWorkerNotifiesHostOfflineOncePerIncident(t *testing.T) {
	db := newAlertTestDB(t)
	node := bytes.Repeat([]byte{4}, 16)
	now := time.Now()
	if err := db.Create(&model.AvailabilityIncident{
		NodeUUID: node, InitialClassification: AlertHostOffline, CurrentClassification: AlertHostOffline,
		Revision: 1, StartedAt: now.UnixNano(), EndedAt: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	notifications := 0
	worker := NewAlertWorker(db, AlertPolicy{NotifyHostOffline: true}, func(string, string) { notifications++ })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if err := worker.Scan(context.Background(), now.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("notifications=%d", notifications)
	}
	var alerts []model.TelemetryAlert
	if err := db.Find(&alerts).Error; err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || !alerts[0].Notified {
		t.Fatalf("alerts=%#v", alerts)
	}
}

func TestAlertWorkerSameClassRevisionDoesNotRenotify(t *testing.T) {
	db := newAlertTestDB(t)
	node := bytes.Repeat([]byte{5}, 16)
	now := time.Now()
	if err := db.Create(&model.AvailabilityIncident{
		NodeUUID: node, InitialClassification: AlertHostOffline, CurrentClassification: AlertHostOffline,
		Revision: 2, StartedAt: now.UnixNano(), EndedAt: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	notifications := 0
	worker := NewAlertWorker(db, AlertPolicy{NotifyHostOffline: true}, func(string, string) { notifications++ })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if notifications != 0 {
		t.Fatalf("notifications=%d", notifications)
	}
}

func TestAlertWorkerCollapsesPerBucketIncidentsToOneEpisode(t *testing.T) {
	db := newAlertTestDB(t)
	node := bytes.Repeat([]byte{6}, 16)
	now := time.Unix(1_700_000_000, 0)
	bucket := int64(30 * time.Second)
	first := now.UnixNano()
	for i := 0; i < 3; i++ {
		start := first + int64(i)*bucket
		if err := db.Create(&model.AvailabilityIncident{
			NodeUUID: node, InitialClassification: AlertHostOffline, CurrentClassification: AlertHostOffline,
			Revision: 1, StartedAt: start, EndedAt: start + bucket,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	notifications := 0
	worker := NewAlertWorker(db, AlertPolicy{NotifyHostOffline: true, AvailabilityBucket: 30 * time.Second}, func(string, string) { notifications++ })
	if err := worker.Scan(context.Background(), now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("notifications=%d", notifications)
	}
	var alerts []model.TelemetryAlert
	if err := db.Find(&alerts).Error; err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || !strings.HasPrefix(alerts[0].DedupKey, "episode/"+AlertHostOffline+"/") {
		t.Fatalf("alerts=%#v", alerts)
	}
}

func TestAlertWorkerBatchesSameTypeInOneScan(t *testing.T) {
	db := newAlertTestDB(t)
	now := time.Now()
	for i := byte(1); i <= 3; i++ {
		node := bytes.Repeat([]byte{i}, 16)
		if err := db.Create(&model.AvailabilityIncident{
			NodeUUID: node, InitialClassification: AlertHostOffline, CurrentClassification: AlertHostOffline,
			Revision: 1, StartedAt: now.UnixNano(), EndedAt: 0,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	var messages []string
	worker := NewAlertWorker(db, AlertPolicy{NotifyHostOffline: true}, func(message, _ string) { messages = append(messages, message) })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || !strings.HasPrefix(messages[0], "[离线] 3 台") {
		t.Fatalf("messages=%#v", messages)
	}
}

func TestAlertWorkerSuppressesHostOfflineWhenHistoryCoversIt(t *testing.T) {
	db := newAlertTestDB(t)
	node := bytes.Repeat([]byte{7}, 16)
	now := time.Now()
	if err := db.Create(&model.AvailabilityIncident{
		NodeUUID: node, InitialClassification: AlertHostOffline, CurrentClassification: AlertHostOffline,
		Revision: 1, StartedAt: now.UnixNano(), EndedAt: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	notifications := 0
	worker := NewAlertWorker(db, AlertPolicy{NotifyHostOffline: true, SuppressHostOfflineNotify: true}, func(string, string) { notifications++ })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if notifications != 0 {
		t.Fatalf("notifications=%d", notifications)
	}
	var alerts []model.TelemetryAlert
	if err := db.Find(&alerts).Error; err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0].Notified {
		t.Fatalf("alerts=%#v", alerts)
	}
}

func TestAlertWorkerCollectorOfflineOncePerLastSeen(t *testing.T) {
	db := newAlertTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	createCollector(t, db, "collector-east", "East")
	lastSeen := now.Add(-3 * time.Minute).UnixNano()
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "collector-east", Status: "offline", LastSeen: lastSeen}).Error; err != nil {
		t.Fatal(err)
	}
	notifications := 0
	worker := NewAlertWorker(db, AlertPolicy{NotifyCollectorOffline: true, CollectorTimeout: 90 * time.Second}, func(string, string) { notifications++ })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if err := worker.Scan(context.Background(), now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("notifications=%d", notifications)
	}
	if err := db.Model(&model.CollectorRuntime{}).Where("collector_uuid = ?", "collector-east").Update("last_seen", now.Add(3*time.Minute).UnixNano()).Error; err != nil {
		t.Fatal(err)
	}
	later := now.Add(5 * time.Minute)
	if err := worker.Scan(context.Background(), later); err != nil {
		t.Fatal(err)
	}
	if notifications != 2 {
		t.Fatalf("after reconnect notifications=%d", notifications)
	}
}

func TestAlertWorkerBatchesCollectorOffline(t *testing.T) {
	db := newAlertTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	lastSeen := now.Add(-3 * time.Minute).UnixNano()
	for _, id := range []string{"collector-a", "collector-b", "collector-c"} {
		createCollector(t, db, id, id)
		if err := db.Create(&model.CollectorRuntime{CollectorUUID: id, Status: "offline", LastSeen: lastSeen}).Error; err != nil {
			t.Fatal(err)
		}
	}
	var messages []string
	worker := NewAlertWorker(db, AlertPolicy{NotifyCollectorOffline: true, CollectorTimeout: 90 * time.Second}, func(message, _ string) { messages = append(messages, message) })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || !strings.HasPrefix(messages[0], "[从端离线] 3 台") || !strings.Contains(messages[0], "collector-a") {
		t.Fatalf("messages=%#v", messages)
	}
}

func TestAlertWorkerHostOfflineUsesServerName(t *testing.T) {
	db := newAlertTestDB(t)
	node := bytes.Repeat([]byte{8}, 16)
	now := time.Now()
	createBoundServer(t, db, node, "东京-1")
	if err := db.Create(&model.AvailabilityIncident{
		NodeUUID: node, InitialClassification: AlertHostOffline, CurrentClassification: AlertHostOffline,
		Revision: 1, StartedAt: now.UnixNano(), EndedAt: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	var messages []string
	worker := NewAlertWorker(db, AlertPolicy{NotifyHostOffline: true}, func(message, _ string) { messages = append(messages, message) })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0] != "[离线] 东京-1" {
		t.Fatalf("messages=%#v", messages)
	}
}

func TestAlertWorkerCollectorOfflineUsesName(t *testing.T) {
	db := newAlertTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	createCollector(t, db, "collector-east", "大阪从端")
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "collector-east", Status: "offline", LastSeen: now.Add(-3 * time.Minute).UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	var messages []string
	worker := NewAlertWorker(db, AlertPolicy{NotifyCollectorOffline: true, CollectorTimeout: 90 * time.Second}, func(message, _ string) { messages = append(messages, message) })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0] != "[从端离线] 大阪从端" {
		t.Fatalf("messages=%#v", messages)
	}
}

func TestAlertWorkerCollectorOnlineOncePerEpisode(t *testing.T) {
	db := newAlertTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	createCollector(t, db, "collector-east", "大阪从端")
	stale := now.Add(-3 * time.Minute).UnixNano()
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "collector-east", Status: "offline", LastSeen: stale}).Error; err != nil {
		t.Fatal(err)
	}
	var messages []string
	worker := NewAlertWorker(db, AlertPolicy{NotifyCollectorOffline: true, NotifyCollectorOnline: true, CollectorTimeout: 90 * time.Second}, func(message, _ string) { messages = append(messages, message) })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	recovered := now.Add(10 * time.Second).UnixNano()
	if err := db.Model(&model.CollectorRuntime{}).Where("collector_uuid = ?", "collector-east").Updates(map[string]any{"last_seen": recovered, "status": "online"}).Error; err != nil {
		t.Fatal(err)
	}
	later := now.Add(20 * time.Second)
	if err := worker.Scan(context.Background(), later); err != nil {
		t.Fatal(err)
	}
	if err := worker.Scan(context.Background(), later.Add(15*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0] != "[从端离线] 大阪从端" || messages[1] != "[从端上线] 大阪从端" {
		t.Fatalf("messages=%#v", messages)
	}
	var alerts []model.TelemetryAlert
	if err := db.Where("alert_type = ?", AlertCollectorOnline).Find(&alerts).Error; err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0].Severity != "info" || !alerts[0].Notified || !strings.HasPrefix(alerts[0].DedupKey, "collector-online/collector-east/") {
		t.Fatalf("alerts=%#v", alerts)
	}
}

func TestAlertWorkerCollectorOnlineSkipsNeverOffline(t *testing.T) {
	db := newAlertTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	createCollector(t, db, "collector-east", "大阪从端")
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "collector-east", Status: "online", LastSeen: now.UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	notifications := 0
	worker := NewAlertWorker(db, AlertPolicy{NotifyCollectorOnline: true, CollectorTimeout: 90 * time.Second}, func(string, string) { notifications++ })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if notifications != 0 {
		t.Fatalf("notifications=%d", notifications)
	}
	var alerts []model.TelemetryAlert
	if err := db.Find(&alerts).Error; err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 0 {
		t.Fatalf("alerts=%#v", alerts)
	}
}

func TestAlertWorkerCollectorOnlineMutedStillPersists(t *testing.T) {
	db := newAlertTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	createCollector(t, db, "collector-east", "大阪从端")
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "collector-east", Status: "offline", LastSeen: now.Add(-3 * time.Minute).UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	notifications := 0
	worker := NewAlertWorker(db, AlertPolicy{NotifyCollectorOffline: true, CollectorTimeout: 90 * time.Second}, func(string, string) { notifications++ })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.CollectorRuntime{}).Where("collector_uuid = ?", "collector-east").Update("last_seen", now.Add(10*time.Second).UnixNano()).Error; err != nil {
		t.Fatal(err)
	}
	if err := worker.Scan(context.Background(), now.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("notifications=%d", notifications)
	}
	var alerts []model.TelemetryAlert
	if err := db.Where("alert_type = ?", AlertCollectorOnline).Find(&alerts).Error; err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0].Notified {
		t.Fatalf("alerts=%#v", alerts)
	}
}

func TestAlertWorkerBatchesCollectorOnline(t *testing.T) {
	db := newAlertTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	lastSeen := now.Add(-3 * time.Minute).UnixNano()
	for _, id := range []string{"collector-a", "collector-b", "collector-c"} {
		createCollector(t, db, id, id)
		if err := db.Create(&model.CollectorRuntime{CollectorUUID: id, Status: "offline", LastSeen: lastSeen}).Error; err != nil {
			t.Fatal(err)
		}
	}
	var messages []string
	worker := NewAlertWorker(db, AlertPolicy{NotifyCollectorOffline: true, NotifyCollectorOnline: true, CollectorTimeout: 90 * time.Second}, func(message, _ string) { messages = append(messages, message) })
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	recovered := now.Add(10 * time.Second).UnixNano()
	if err := db.Model(&model.CollectorRuntime{}).Where("collector_uuid IN ?", []string{"collector-a", "collector-b", "collector-c"}).Update("last_seen", recovered).Error; err != nil {
		t.Fatal(err)
	}
	if err := worker.Scan(context.Background(), now.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || !strings.HasPrefix(messages[0], "[从端离线] 3 台") || !strings.HasPrefix(messages[1], "[从端上线] 3 台") || !strings.Contains(messages[1], "collector-a") {
		t.Fatalf("messages=%#v", messages)
	}
}

func TestAlertWorkerReadsPolicyEachScan(t *testing.T) {
	db := newAlertTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	createCollector(t, db, "collector-east", "East")
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "collector-east", Status: "offline", LastSeen: now.Add(-3 * time.Minute).UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	policy := AlertPolicy{CollectorTimeout: 90 * time.Second}
	notifications := 0
	worker := NewAlertWorkerFrom(db, func() AlertPolicy { return policy }, func(string, string) { notifications++ })
	policy.NotifyCollectorOffline = true
	if err := worker.Scan(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("notifications=%d", notifications)
	}
}

func TestDisplayHostNameAvoidsFullHash(t *testing.T) {
	node := bytes.Repeat([]byte{0xab}, 16)
	got := displayHostName("", node)
	if got != "abababab" {
		t.Fatalf("got=%q", got)
	}
}

func createBoundServer(t *testing.T, db *gorm.DB, node []byte, name string) {
	t.Helper()
	server := model.Server{Name: name, Secret: "secret"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ServerNodeBinding{ServerID: server.ID, NodeUUID: node, Current: true, Reason: "test", ValidFrom: time.Now().UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
}

func newAlertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AvailabilityIncident{}, &model.Collector{}, &model.CollectorRuntime{}, &model.TelemetryDataLoss{}, &model.TelemetryEvent{}, &model.TelemetryAlert{}, &model.Server{}, &model.ServerNodeBinding{}); err != nil {
		t.Fatal(err)
	}
	return db
}
