package singleton

import (
	"testing"
	"time"

	"github.com/naiba/nezha/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestDB 创建内存 sqlite 并迁移离线检测所需的两张表。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.ServerRuntime{}, &model.ServerOfflineHistory{}); err != nil {
		t.Fatalf("自动迁移失败: %v", err)
	}
	// 清空表，避免 cache=shared 模式下残留
	db.Exec("DELETE FROM server_offline_histories")
	db.Exec("DELETE FROM server_runtimes")
	return db
}

// insertClosedHistory 插入一条已关闭的离线历史（自增 ID），返回写入后的记录。
// lastSeenAt 默认等于 startedAt（即断线前最后一次上报时间）；StartedAt 在真实逻辑里 = LastSeenAt + 阈值。
func insertClosedHistory(t *testing.T, db *gorm.DB, serverID uint64, startedAt, endedAt time.Time, reason string) model.ServerOfflineHistory {
	t.Helper()
	end := endedAt
	h := model.ServerOfflineHistory{
		ServerID:         serverID,
		StartedAt:        startedAt,
		LastSeenAt:       startedAt,
		EndedAt:          &end,
		RecoveredAt:      &end,
		Status:           model.OfflineHistoryStatusClosed,
		Reason:           reason,
		DurationSeconds:  uint64(endedAt.Sub(startedAt).Seconds()),
		ThresholdSeconds: 30,
	}
	if err := db.Create(&h).Error; err != nil {
		t.Fatalf("插入历史失败: %v", err)
	}
	return h
}

func TestFormatAvailabilityPercent(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 0},
		{-1, 0},
		{100, 100},
		{150, 100},
		{99.999, 99.99}, // 向下取整，不为 100
		{99.995, 99.99},
		{95.4567, 95.45},
		{50.0, 50},
	}
	for _, c := range cases {
		got := FormatAvailabilityPercent(c.in)
		if got != c.want {
			t.Errorf("FormatAvailabilityPercent(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDetectOfflineReason(t *testing.T) {
	cases := []struct {
		name                               string
		lastBoot, recoveredBoot, lastUp, recoveredUp uint64
		want                               string
	}{
		{"zero_boot_unknown", 0, 100, 10, 20, model.OfflineReasonUnknown},
		{"reboot_smaller_unknown", 200, 100, 10, 20, model.OfflineReasonUnknown},
		{"machine_reboot_higher_boot", 100, 200, 10, 20, model.OfflineReasonMachineReboot},
		{"machine_reboot_lower_uptime", 100, 100, 30, 10, model.OfflineReasonMachineReboot},
		{"network_disconnect", 100, 100, 10, 20, model.OfflineReasonNetworkDisconnect},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectOfflineReason(c.lastBoot, c.recoveredBoot, c.lastUp, c.recoveredUp)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestTryMergeWithPrevious_MergesWithinGap：相邻两次离线间隔 < gap，合并为一条。
func TestTryMergeWithPrevious_MergesWithinGap(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{OfflineMergeGapSeconds: 60}

	serverID := uint64(100)
	prevStart := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	prevEnd := time.Date(2026, 6, 1, 10, 1, 0, 0, time.UTC) // 上一条结束
	insertClosedHistory(t, DB, serverID, prevStart, prevEnd, model.OfflineReasonNetworkDisconnect)

	// 当前记录：断线点在 10:01:05（距上一条恢复仅 5 秒，< 60s）
	curStart := prevEnd.Add(5 * time.Second)
	curEnd := curStart.Add(30 * time.Second)
	current := insertClosedHistory(t, DB, serverID, curStart, curEnd, model.OfflineReasonNetworkDisconnect)

	tryMergeWithPrevious(serverID, &current)

	var remain []model.ServerOfflineHistory
	DB.Find(&remain)
	if len(remain) != 1 {
		t.Fatalf("期望合并后剩 1 条，实际 %d", len(remain))
	}
	merged := remain[0]
	if merged.ID == current.ID {
		t.Fatal("当前记录应被删除，却仍存在")
	}
	if merged.EndedAt == nil || !merged.EndedAt.Equal(curEnd) {
		t.Errorf("合并后结束时间应为当前记录的结束时间 %v，实际 %v", curEnd, merged.EndedAt)
	}
	if merged.StartedAt.Equal(prevStart) == false {
		t.Errorf("合并后开始时间应保持上一条的 %v，实际 %v", prevStart, merged.StartedAt)
	}
	wantDur := uint64(curEnd.Sub(prevStart).Seconds())
	if merged.DurationSeconds != wantDur {
		t.Errorf("合并后时长应为 %d，实际 %d", wantDur, merged.DurationSeconds)
	}
}

// TestTryMergeWithPrevious_NoMergeBeyondGap：间隔 > gap，两条都保留。
func TestTryMergeWithPrevious_NoMergeBeyondGap(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{OfflineMergeGapSeconds: 60}

	serverID := uint64(101)
	prevStart := time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC)
	prevEnd := time.Date(2026, 6, 1, 11, 1, 0, 0, time.UTC)
	insertClosedHistory(t, DB, serverID, prevStart, prevEnd, model.OfflineReasonNetworkDisconnect)

	// 当前记录断线点距上一条恢复 120 秒，> 60s，不合并
	curStart := prevEnd.Add(120 * time.Second)
	curEnd := curStart.Add(30 * time.Second)
	current := insertClosedHistory(t, DB, serverID, curStart, curEnd, model.OfflineReasonNetworkDisconnect)

	tryMergeWithPrevious(serverID, &current)

	var remain []model.ServerOfflineHistory
	DB.Find(&remain)
	if len(remain) != 2 {
		t.Fatalf("期望不合并剩 2 条，实际 %d", len(remain))
	}
}

// TestTryMergeWithPrevious_ReasonMismatchDowngrade：两段原因不同时降级为 unknown。
func TestTryMergeWithPrevious_ReasonMismatchDowngrade(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{OfflineMergeGapSeconds: 60}

	serverID := uint64(102)
	prevStart := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	prevEnd := time.Date(2026, 6, 1, 12, 1, 0, 0, time.UTC)
	insertClosedHistory(t, DB, serverID, prevStart, prevEnd, model.OfflineReasonNetworkDisconnect)

	curStart := prevEnd.Add(5 * time.Second)
	curEnd := curStart.Add(30 * time.Second)
	current := insertClosedHistory(t, DB, serverID, curStart, curEnd, model.OfflineReasonMachineReboot)

	tryMergeWithPrevious(serverID, &current)

	var remain []model.ServerOfflineHistory
	DB.Find(&remain)
	if len(remain) != 1 {
		t.Fatalf("期望合并后剩 1 条，实际 %d", len(remain))
	}
	if remain[0].Reason != model.OfflineReasonUnknown {
		t.Errorf("原因不同时应降级为 unknown，实际 %q", remain[0].Reason)
	}
}

// TestTryMergeWithPrevious_NoPrevious：没有上一条记录，不合并、不报错。
func TestTryMergeWithPrevious_NoPrevious(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{OfflineMergeGapSeconds: 60}

	serverID := uint64(103)
	curStart := time.Date(2026, 6, 1, 13, 0, 0, 0, time.UTC)
	curEnd := curStart.Add(30 * time.Second)
	current := insertClosedHistory(t, DB, serverID, curStart, curEnd, model.OfflineReasonNetworkDisconnect)

	// 不应 panic，记录原样保留
	tryMergeWithPrevious(serverID, &current)

	var remain []model.ServerOfflineHistory
	DB.Find(&remain)
	if len(remain) != 1 || remain[0].ID != current.ID {
		t.Fatalf("无上一条时当前记录应原样保留，实际 %+v", remain)
	}
}

// TestTryMergeWithPrevious_ThresholdNotCountedAsOnline：合并窗口按真实在线时间判断，
// 不应把离线阈值（StartedAt - LastSeenAt）算进在线时间。
// 场景：阈值=60s、merge gap=30s。LastSeenAt 距上次恢复 5s（< gap，应合并），
// 但 StartedAt(=LastSeenAt+60s) 距上次恢复 65s（> gap，旧错误逻辑会漏合并）。
func TestTryMergeWithPrevious_ThresholdNotCountedAsOnline(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{OfflineMergeGapSeconds: 30}

	serverID := uint64(104)
	prevStart := time.Date(2026, 6, 1, 14, 0, 0, 0, time.UTC)
	prevEnd := time.Date(2026, 6, 1, 14, 1, 0, 0, time.UTC) // 上次恢复点
	insertClosedHistory(t, DB, serverID, prevStart, prevEnd, model.OfflineReasonNetworkDisconnect)

	// 当前记录：最后一次上报距上次恢复 5s（真实在线 5s < gap 30s）
	lastSeen := prevEnd.Add(5 * time.Second)
	threshold := 60 * time.Second
	startedAt := lastSeen.Add(threshold) // StartedAt = LastSeenAt + 阈值 = 距上次恢复 65s
	curEnd := startedAt.Add(30 * time.Second)
	current := insertClosedHistory(t, DB, serverID, startedAt, curEnd, model.OfflineReasonNetworkDisconnect)
	// 让 LastSeenAt 反映真实最后一次上报（insertClosedHistory 默认设为 StartedAt，这里修正）
	current.LastSeenAt = lastSeen
	if err := DB.Model(&model.ServerOfflineHistory{}).Where("id = ?", current.ID).
		Update("last_seen_at", lastSeen).Error; err != nil {
		t.Fatalf("更新 LastSeenAt 失败: %v", err)
	}

	tryMergeWithPrevious(serverID, &current)

	var remain []model.ServerOfflineHistory
	DB.Find(&remain)
	if len(remain) != 1 {
		t.Fatalf("真实在线 5s < gap 30s，应合并为 1 条，实际 %d（阈值被错误计入在线时间）", len(remain))
	}
}

// TestTryMergeWithPrevious_ReturnsMergedRecord：合并成功时返回延展后的上一条记录，
// 保证 CloseOfflineHistory 能据此发送与落库一致的恢复通知。
func TestTryMergeWithPrevious_ReturnsMergedRecord(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{OfflineMergeGapSeconds: 60}

	serverID := uint64(105)
	prevStart := time.Date(2026, 6, 1, 15, 0, 0, 0, time.UTC)
	prevEnd := time.Date(2026, 6, 1, 15, 1, 0, 0, time.UTC)
	insertClosedHistory(t, DB, serverID, prevStart, prevEnd, model.OfflineReasonNetworkDisconnect)

	curStart := prevEnd.Add(5 * time.Second)
	curEnd := curStart.Add(30 * time.Second)
	current := insertClosedHistory(t, DB, serverID, curStart, curEnd, model.OfflineReasonNetworkDisconnect)

	final := tryMergeWithPrevious(serverID, &current)
	if final == nil {
		t.Fatal("返回值不应为 nil")
	}
	if final.ID == current.ID {
		t.Errorf("合并成功应返回上一条记录，却返回了当前（已删除）记录")
	}
	if final.EndedAt == nil || !final.EndedAt.Equal(curEnd) {
		t.Errorf("返回记录的结束时间应为合并后的 %v，实际 %v", curEnd, final.EndedAt)
	}
}

// TestInitServerRuntimes_NoSyntheticTimestamps：注册在 ServerList 但 Agent 从未上报的服务器，
// InitServerRuntimes 只应创建骨架行（Status=unknown、LastSeenAt=nil），
// 不应伪造 LastSeenAt/LastOnlineAt——否则前台可用性会错误显示 100%（应为空值）。
func TestInitServerRuntimes_NoSyntheticTimestamps(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{}

	serverID := uint64(300)
	// 模拟服务器已注册但 Agent 从未连接
	ServerLock.Lock()
	ServerList = map[uint64]*model.Server{serverID: {Common: model.Common{ID: serverID}}}
	ServerLock.Unlock()
	defer func() {
		ServerLock.Lock()
		delete(ServerList, serverID)
		ServerLock.Unlock()
	}()

	InitServerRuntimes()

	var rt model.ServerRuntime
	if err := DB.Where("server_id = ?", serverID).First(&rt).Error; err != nil {
		t.Fatalf("应创建骨架运行态记录: %v", err)
	}
	if rt.LastSeenAt != nil {
		t.Errorf("未上报服务器不应伪造 LastSeenAt，实际 %v（会导致可用性误判为 100%%）", *rt.LastSeenAt)
	}
	if rt.LastOnlineAt != nil {
		t.Errorf("未上报服务器不应伪造 LastOnlineAt，实际 %v", *rt.LastOnlineAt)
	}
	if rt.Status != model.ServerRuntimeStatusUnknown {
		t.Errorf("未上报服务器状态应为 unknown，实际 %q", rt.Status)
	}

	// 验证可用性判定：该服务器应为空值
	summaries, _, err := GetServerAvailabilitySummaries([]uint64{serverID}, 30)
	if err != nil {
		t.Fatalf("查询可用性失败: %v", err)
	}
	if s := summaries[serverID]; s == nil || s.AvailabilityPercent != nil {
		t.Fatalf("从未上报的服务器可用性应为 nil，实际 %+v", summaries[serverID])
	}
}
