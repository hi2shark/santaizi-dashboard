package singleton

import (
	"testing"
	"time"

	"github.com/naiba/nezha/model"
)

// insertRuntime 插入一条 ServerRuntime。
// lastSeenAt == nil 表示从未上报（含 InitServerRuntimes/GetOrCreateServerRuntime 创建后尚未上报的情况）。
// 注意：是否上报以 lastSeenAt 为准，firstSeenAt 在创建路径中常不写入，不能作为可靠信号。
func insertRuntime(t *testing.T, serverID uint64, lastSeenAt *time.Time) {
	t.Helper()
	rt := model.ServerRuntime{
		ServerID:  serverID,
		Status:    model.ServerRuntimeStatusOnline,
		LastSeenAt: lastSeenAt,
	}
	if err := DB.Create(&rt).Error; err != nil {
		t.Fatalf("插入运行态失败: %v", err)
	}
}

// TestGetServerAvailabilitySummaries_NeverReported：从未上报的服务器可用性应为 nil，而非 100%。
func TestGetServerAvailabilitySummaries_NeverReported(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{}

	serverID := uint64(200)
	// LastSeenAt 为 nil → 从未上报
	insertRuntime(t, serverID, nil)

	summaries, _, err := GetServerAvailabilitySummaries([]uint64{serverID}, 30)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	s, ok := summaries[serverID]
	if !ok {
		t.Fatal("结果中应包含该服务器")
	}
	if s.AvailabilityPercent != nil {
		t.Errorf("从未上报的服务器可用性应为 nil，实际 %v", *s.AvailabilityPercent)
	}
}

// TestGetServerAvailabilitySummaries_ReportedNoOffline：已上报且无离线历史，可用性应为 100。
func TestGetServerAvailabilitySummaries_ReportedNoOffline(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{}

	serverID := uint64(201)
	now := time.Now()
	insertRuntime(t, serverID, &now)

	summaries, _, err := GetServerAvailabilitySummaries([]uint64{serverID}, 30)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	s, ok := summaries[serverID]
	if !ok {
		t.Fatal("结果中应包含该服务器")
	}
	if s.AvailabilityPercent == nil {
		t.Fatal("已上报的服务器可用性不应为 nil")
	}
	if *s.AvailabilityPercent != 100 {
		t.Errorf("无离线历史时可用性应为 100，实际 %v", *s.AvailabilityPercent)
	}
}

// TestGetServerAvailabilitySummaries_NoRuntimeTreatedAsUnreported：
// 防御性：缺少运行态记录（理论上应已由 InitServerRuntimes 创建）也按未上报处理。
func TestGetServerAvailabilitySummaries_NoRuntimeTreatedAsUnreported(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{}

	serverID := uint64(202)
	// 不插入任何 runtime 记录

	summaries, _, err := GetServerAvailabilitySummaries([]uint64{serverID}, 30)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	s, ok := summaries[serverID]
	if !ok {
		t.Fatal("结果中应包含该服务器")
	}
	if s.AvailabilityPercent != nil {
		t.Errorf("缺少运行态应按未上报处理（nil），实际 %v", *s.AvailabilityPercent)
	}
}

// TestGetServerAvailabilitySummaries_LegacyRuntimeWithoutFirstSeenAt：
// 兼容已有数据：旧版本（或 InitServerRuntimes / GetOrCreateServerRuntime 创建）的运行态只写了
// LastSeenAt 而未写 FirstSeenAt。只要 LastSeenAt 非空，就应判定为已上报、给出真实可用性，
// 不能因 FirstSeenAt 缺失而误判为“从未上报”显示空值。
func TestGetServerAvailabilitySummaries_LegacyRuntimeWithoutFirstSeenAt(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{}

	serverID := uint64(203)
	now := time.Now()
	// 模拟旧数据：有 LastSeenAt、无 FirstSeenAt（InitServerRuntimes 的产物）
	rt := model.ServerRuntime{
		ServerID:   serverID,
		Status:     model.ServerRuntimeStatusOnline,
		LastSeenAt: &now,
		// FirstSeenAt 故意留空
	}
	if err := DB.Create(&rt).Error; err != nil {
		t.Fatalf("插入运行态失败: %v", err)
	}

	summaries, _, err := GetServerAvailabilitySummaries([]uint64{serverID}, 30)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	s, ok := summaries[serverID]
	if !ok {
		t.Fatal("结果中应包含该服务器")
	}
	if s.AvailabilityPercent == nil {
		t.Fatal("有 LastSeenAt 的旧数据服务器应判定为已上报（非 nil），不能因缺 FirstSeenAt 误判为空")
	}
	if *s.AvailabilityPercent != 100 {
		t.Errorf("无离线历史时可用性应为 100，实际 %v", *s.AvailabilityPercent)
	}
}

// TestAvailability_OfflineHistoryDisabledButReporting：
// 关键回归：当 EnableOfflineHistory=false（运行态更新历史上会整体短路）但 Agent 仍在正常上报、
// 且 ShowAvailabilityToGuest=true 时，可用性接口仍会开放。此时活跃上报的服务器不应被判成
// “未上报”而显示空值。修复后 UpdateServerRuntimeOn*Report 解耦了上报时间戳写入与离线历史，
// LastSeenAt 始终被写入 → 可用性应为真实值（非空）。
func TestAvailability_OfflineHistoryDisabledButReporting(t *testing.T) {
	DB = newTestDB(t)
	Conf = &model.Config{EnableOfflineHistory: false} // 离线历史关闭

	serverID := uint64(204)
	// 模拟 Agent 正常上报（即使离线历史关闭，运行态时间戳也应被写入）
	UpdateServerRuntimeOnStateReport(serverID, model.HostState{Uptime: 100})

	// 核对：运行态 LastSeenAt 必须非空
	var rt model.ServerRuntime
	if err := DB.Where("server_id = ?", serverID).First(&rt).Error; err != nil {
		t.Fatalf("运行态应已创建: %v", err)
	}
	if rt.LastSeenAt == nil {
		t.Fatal("离线历史关闭时 Agent 上报后 LastSeenAt 仍应被写入，否则可用性会误判为空")
	}

	// 可用性应非空（已上报、无离线历史 → 100）
	summaries, _, err := GetServerAvailabilitySummaries([]uint64{serverID}, 30)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	s, ok := summaries[serverID]
	if !ok {
		t.Fatal("结果中应包含该服务器")
	}
	if s.AvailabilityPercent == nil {
		t.Fatal("离线历史关闭但 Agent 正常上报时，可用性不应为空（修复前因运行态更新短路会误判）")
	}
	if *s.AvailabilityPercent != 100 {
		t.Errorf("无离线历史时可用性应为 100，实际 %v", *s.AvailabilityPercent)
	}
}
