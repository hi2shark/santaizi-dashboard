package singleton

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newV2DetectorDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(
		&model.ServerRuntime{}, &model.ServerOfflineHistory{},
		&model.AvailabilityBucket{}, &model.ObserverPathBucket{}, &model.Collector{},
	); err != nil {
		t.Fatalf("自动迁移失败: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB(db) })
	return db
}

func setupV2Detector(t *testing.T) {
	t.Helper()
	previousDB, previousConf := DB, Conf
	DB = newV2DetectorDB(t)
	Conf = &model.Config{
		EnableOfflineHistory:        true,
		OfflineThresholdSeconds:     60,
		OfflineCheckIntervalSeconds: 10,
		EnableOfflineNotification:   false,
		EnableRecoveryNotification:  false,
		Telemetry:                   model.TelemetryConfig{AvailabilityBucketSeconds: 30},
	}
	offlineDetectorStartTime = time.Now().Add(-time.Hour)
	t.Cleanup(func() {
		DB = previousDB
		Conf = previousConf
	})
}

func v2TestNode(id byte) []byte {
	node := make([]byte, 16)
	node[15] = id
	return node
}

func alignedBucket(now time.Time, ago int) int64 {
	size := int64(30 * time.Second)
	latest := now.UnixNano() / size * size
	return latest - int64(ago)*size
}

func insertV2Runtime(t *testing.T, serverID uint64, node []byte, status string, lastSeen time.Time, offlineID uint64) {
	t.Helper()
	rt := model.ServerRuntime{
		ServerID:         serverID,
		Status:           status,
		Protocol:         v2Protocol,
		CurrentNodeUUID:  append([]byte(nil), node...),
		LastSeenAt:       &lastSeen,
		CurrentOfflineID: offlineID,
		LastBootTime:     100,
		LastUptime:       50,
		LastIP:           "10.0.0.1",
	}
	if err := DB.Create(&rt).Error; err != nil {
		t.Fatalf("插入运行态失败: %v", err)
	}
}

func insertBucket(t *testing.T, node []byte, start int64, host, connectivity string, expected, healthy, seen uint32, summary []byte) {
	t.Helper()
	row := model.AvailabilityBucket{
		NodeUUID: node, BucketStart: start, HostState: host, ConnectivityState: connectivity,
		ExpectedObservers: expected, HealthyObservers: healthy, SeenObservers: seen,
		ObserverSummary: summary, Revision: 1,
	}
	if err := DB.Create(&row).Error; err != nil {
		t.Fatalf("插入观测桶失败: %v", err)
	}
}

func insertPath(t *testing.T, node []byte, observer string, start, lastSeen int64, seen bool) {
	t.Helper()
	row := model.ObserverPathBucket{
		NodeUUID: node, ObserverID: observer, BucketStart: start, Seen: seen, LastSeenAt: lastSeen, Revision: 1,
	}
	if err := DB.Create(&row).Error; err != nil {
		t.Fatalf("插入路径桶失败: %v", err)
	}
}

func evidenceJSON(t *testing.T, rows []v2ObserverEvidence) []byte {
	t.Helper()
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("编码观测证据失败: %v", err)
	}
	return raw
}

func openHistoryCount(t *testing.T, serverID uint64) int64 {
	t.Helper()
	var n int64
	if err := DB.Model(&model.ServerOfflineHistory{}).
		Where("server_id = ? AND status = ?", serverID, model.OfflineHistoryStatusOpen).
		Count(&n).Error; err != nil {
		t.Fatalf("统计离线记录失败: %v", err)
	}
	return n
}

func TestDetectOfflineServers_V2PrimaryDownCollectorSeenDoesNotOpen(t *testing.T) {
	setupV2Detector(t)
	now := time.Now()
	node := v2TestNode(1)
	stale := now.Add(-2 * time.Hour)
	insertV2Runtime(t, 1, node, model.ServerRuntimeStatusOnline, stale, 0)
	summary := evidenceJSON(t, []v2ObserverEvidence{
		{ObserverID: primaryObserverID, Healthy: true, Seen: false},
		{ObserverID: "collector-east", Healthy: true, Seen: true},
	})
	insertBucket(t, node, alignedBucket(now, 0), model.HostStateOnline, model.ConnectivityPartial, 2, 2, 1, summary)
	insertBucket(t, node, alignedBucket(now, 1), model.HostStateOnline, model.ConnectivityPartial, 2, 2, 1, summary)

	DetectOfflineServers()

	if openHistoryCount(t, 1) != 0 {
		t.Fatal("主端断、从端仍看到时不应开离线记录")
	}
}

func TestDetectOfflineServers_V2CollectorDownPrimarySeenDoesNotOpen(t *testing.T) {
	setupV2Detector(t)
	now := time.Now()
	node := v2TestNode(2)
	insertV2Runtime(t, 2, node, model.ServerRuntimeStatusOnline, now, 0)
	summary := evidenceJSON(t, []v2ObserverEvidence{
		{ObserverID: primaryObserverID, Healthy: true, Seen: true},
		{ObserverID: "collector-east", Healthy: true, Seen: false},
	})
	insertBucket(t, node, alignedBucket(now, 0), model.HostStateOnline, model.ConnectivityPartial, 2, 2, 1, summary)

	DetectOfflineServers()

	if openHistoryCount(t, 2) != 0 {
		t.Fatal("从端断、主端仍看到时不应开离线记录")
	}
}

func TestDetectOfflineServers_V2AllObserversMissOpensHistory(t *testing.T) {
	setupV2Detector(t)
	now := time.Now()
	node := v2TestNode(3)
	lastReport := time.Unix(0, alignedBucket(now, 3)+int64(5*time.Second))
	insertV2Runtime(t, 3, node, model.ServerRuntimeStatusOnline, lastReport, 0)
	summary := evidenceJSON(t, []v2ObserverEvidence{
		{ObserverID: primaryObserverID, Healthy: true, Seen: false},
		{ObserverID: "collector-east", Healthy: true, Seen: false},
	})
	since := alignedBucket(now, 2)
	insertBucket(t, node, alignedBucket(now, 0), model.HostStateOffline, model.ConnectivityUnavailable, 2, 2, 0, summary)
	insertBucket(t, node, alignedBucket(now, 1), model.HostStateOffline, model.ConnectivityUnavailable, 2, 2, 0, summary)
	insertBucket(t, node, since, model.HostStateOffline, model.ConnectivityUnavailable, 2, 2, 0, summary)
	insertPath(t, node, primaryObserverID, alignedBucket(now, 3), lastReport.UnixNano(), true)
	insertPath(t, node, "collector-east", alignedBucket(now, 3), lastReport.UnixNano(), true)

	DetectOfflineServers()

	if openHistoryCount(t, 3) != 1 {
		t.Fatal("全部健康观测点都看不到时应开一条离线记录")
	}
	var history model.ServerOfflineHistory
	if err := DB.Where("server_id = ?", 3).First(&history).Error; err != nil {
		t.Fatalf("读取离线历史失败: %v", err)
	}
	if !history.StartedAt.Equal(time.Unix(0, since)) {
		t.Errorf("StartedAt 应为连续离线起点 %v，实际 %v", time.Unix(0, since), history.StartedAt)
	}
	if !history.LastSeenAt.Equal(lastReport) {
		t.Errorf("LastSeenAt 应为跨观测点最后上报 %v，实际 %v", lastReport, history.LastSeenAt)
	}
	var rt model.ServerRuntime
	if err := DB.First(&rt, 3).Error; err != nil {
		t.Fatalf("读取运行态失败: %v", err)
	}
	if rt.Status != model.ServerRuntimeStatusOffline || rt.CurrentOfflineID != history.ID {
		t.Errorf("运行态应为 offline 并指向记录，实际 status=%s id=%d", rt.Status, rt.CurrentOfflineID)
	}
}

func TestDetectOfflineServers_V2AnyObserverSeenClosesHistory(t *testing.T) {
	setupV2Detector(t)
	now := time.Now()
	node := v2TestNode(4)
	started := time.Unix(0, alignedBucket(now, 4))
	lastSeen := started.Add(-30 * time.Second)
	rt := model.ServerRuntime{
		ServerID: serverIDForCloseTest(), Status: model.ServerRuntimeStatusOffline, Protocol: v2Protocol,
		CurrentNodeUUID: append([]byte(nil), node...), LastSeenAt: &lastSeen,
		LastBootTime: 100, LastUptime: 50,
	}
	if err := DB.Create(&rt).Error; err != nil {
		t.Fatalf("插入运行态失败: %v", err)
	}
	open := model.ServerOfflineHistory{
		ServerID: rt.ServerID, StartedAt: started, DetectedAt: started.Add(time.Minute),
		Status: model.OfflineHistoryStatusOpen, LastSeenAt: lastSeen, LastBootTime: 100, LastUptime: 50,
	}
	if err := DB.Create(&open).Error; err != nil {
		t.Fatalf("插入离线历史失败: %v", err)
	}
	rt.CurrentOfflineID = open.ID
	if err := DB.Save(&rt).Error; err != nil {
		t.Fatalf("更新运行态失败: %v", err)
	}
	summary := evidenceJSON(t, []v2ObserverEvidence{
		{ObserverID: primaryObserverID, Healthy: true, Seen: false},
		{ObserverID: "collector-east", Healthy: true, Seen: true},
	})
	insertBucket(t, node, alignedBucket(now, 0), model.HostStateOnline, model.ConnectivityPartial, 2, 2, 1, summary)

	DetectOfflineServers()

	var got model.ServerOfflineHistory
	if err := DB.First(&got, open.ID).Error; err != nil {
		t.Fatalf("读取离线历史失败: %v", err)
	}
	if got.Status != model.OfflineHistoryStatusClosed || got.EndedAt == nil {
		t.Fatalf("任一观测点重新看到后应关闭记录，实际 status=%q ended=%v", got.Status, got.EndedAt)
	}
	var gotRt model.ServerRuntime
	if err := DB.First(&gotRt, rt.ServerID).Error; err != nil {
		t.Fatalf("读取运行态失败: %v", err)
	}
	if gotRt.Status != model.ServerRuntimeStatusOnline || gotRt.CurrentOfflineID != 0 {
		t.Errorf("恢复后运行态应为 online，实际 status=%s id=%d", gotRt.Status, gotRt.CurrentOfflineID)
	}
}

func serverIDForCloseTest() uint64 { return 4 }

func TestDetectOfflineServers_V2UnhealthyCollectorDoesNotForceOffline(t *testing.T) {
	setupV2Detector(t)
	now := time.Now()
	node := v2TestNode(5)
	stale := now.Add(-2 * time.Hour)
	insertV2Runtime(t, 5, node, model.ServerRuntimeStatusOnline, stale, 0)
	summary := evidenceJSON(t, []v2ObserverEvidence{
		{ObserverID: primaryObserverID, Healthy: true, Seen: true},
		{ObserverID: "collector-dead", Healthy: false, Seen: false},
	})
	insertBucket(t, node, alignedBucket(now, 0), model.HostStateOnline, model.ConnectivityFull, 2, 1, 1, summary)
	insertBucket(t, node, alignedBucket(now, 1), model.HostStateOnline, model.ConnectivityFull, 2, 1, 1, summary)
	insertBucket(t, node, alignedBucket(now, 2), model.HostStateOnline, model.ConnectivityFull, 2, 1, 1, summary)

	DetectOfflineServers()

	if openHistoryCount(t, 5) != 0 {
		t.Fatal("不健康从端不应把主机打成离线")
	}
}

func TestDetectOfflineServers_V2RecoveringSkipped(t *testing.T) {
	setupV2Detector(t)
	now := time.Now()
	node := v2TestNode(6)
	insertV2Runtime(t, 6, node, model.ServerRuntimeStatusRecovering, now.Add(-time.Hour), 0)
	summary := evidenceJSON(t, []v2ObserverEvidence{
		{ObserverID: primaryObserverID, Healthy: true, Seen: false},
	})
	insertBucket(t, node, alignedBucket(now, 0), model.HostStateOffline, model.ConnectivityUnavailable, 1, 1, 0, summary)
	insertBucket(t, node, alignedBucket(now, 1), model.HostStateOffline, model.ConnectivityUnavailable, 1, 1, 0, summary)
	insertBucket(t, node, alignedBucket(now, 2), model.HostStateOffline, model.ConnectivityUnavailable, 1, 1, 0, summary)

	DetectOfflineServers()

	if openHistoryCount(t, 6) != 0 {
		t.Fatal("主端刚重启的 recovering 节点不应立刻判离线")
	}
}

func TestDetectOfflineServers_V1StillUsesLastSeenAt(t *testing.T) {
	setupV2Detector(t)
	lastSeen := time.Now().Add(-2 * time.Minute)
	rt := model.ServerRuntime{
		ServerID: 7, Status: model.ServerRuntimeStatusOnline, LastSeenAt: &lastSeen,
	}
	if err := DB.Create(&rt).Error; err != nil {
		t.Fatalf("插入 V1 运行态失败: %v", err)
	}

	DetectOfflineServers()

	if openHistoryCount(t, 7) != 1 {
		t.Fatal("V1 超过阈值未上报仍应开离线记录")
	}
}

func TestDetectOfflineServers_V2ShortOfflineDoesNotOpen(t *testing.T) {
	setupV2Detector(t)
	now := time.Now()
	node := v2TestNode(8)
	insertV2Runtime(t, 8, node, model.ServerRuntimeStatusOnline, now.Add(-20*time.Second), 0)
	summary := evidenceJSON(t, []v2ObserverEvidence{
		{ObserverID: primaryObserverID, Healthy: true, Seen: false},
		{ObserverID: "collector-east", Healthy: true, Seen: false},
	})
	insertBucket(t, node, alignedBucket(now, 0), model.HostStateOffline, model.ConnectivityUnavailable, 2, 2, 0, summary)

	DetectOfflineServers()

	if openHistoryCount(t, 8) != 0 {
		t.Fatal("连续离线未达阈值不应开记录")
	}
}

func TestV2ObserverLineFromLatestOfflineBucket(t *testing.T) {
	setupV2Detector(t)
	now := time.Now()
	node := v2TestNode(9)
	insertV2Runtime(t, 9, node, model.ServerRuntimeStatusOffline, now.Add(-time.Minute), 1)
	insertBucket(t, node, alignedBucket(now, 0), model.HostStateOffline, model.ConnectivityUnavailable, 1, 1, 0, evidenceJSON(t, []v2ObserverEvidence{
		{ObserverID: primaryObserverID, Healthy: true, Seen: false},
	}))
	if got := v2ObserverLine(9); got != "观测点：主面板" {
		t.Errorf("通知观测点行应为「观测点：主面板」，实际 %q", got)
	}
}

func TestDurationSecondsBetweenRoundsHalfUp(t *testing.T) {
	start := time.Date(2026, 8, 17, 23, 4, 49, 300_000_000, time.UTC)
	end := start.Add(6*time.Minute + 7*time.Second + 700*time.Millisecond)
	if got := durationSecondsBetween(start, end); got != 368 {
		t.Errorf("367.7s 应四舍五入为 368，实际 %d", got)
	}
	if got := durationSecondsBetween(end, start); got != 0 {
		t.Errorf("倒序时长应为 0，实际 %d", got)
	}
}

func TestObserverDisplayNamesPrimary(t *testing.T) {
	setupV2Detector(t)
	names := observerDisplayNames(evidenceJSON(t, []v2ObserverEvidence{
		{ObserverID: primaryObserverID, Healthy: true, Seen: false},
		{ObserverID: "abcdef0123456789", Healthy: true, Seen: false},
	}))
	if len(names) != 2 || names[0] != "主面板" || names[1] != "abcdef01" {
		t.Errorf("观测点名称不符合预期: %v", names)
	}
}
