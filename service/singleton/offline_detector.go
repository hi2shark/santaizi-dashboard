package singleton

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/naiba/nezha/model"
)

var (
	offlineDetectorStartTime time.Time
	offlineDetectorMu        sync.Mutex
	offlineDetectorCancel    context.CancelFunc
	// offlineDetectorReload 用于在不重启整个检测循环的前提下，让检测间隔等配置变更即时生效。
	offlineDetectorReload chan struct{}
)

// StartOfflineDetector 启动或重启离线检测任务。
// 当配置变更（如启用/禁用离线历史、修改检测间隔）时，可重复调用以应用新配置。
func StartOfflineDetector() {
	offlineDetectorMu.Lock()
	defer offlineDetectorMu.Unlock()

	// 如果已有检测循环在跑，先停止它，以便应用新的配置
	if offlineDetectorCancel != nil {
		offlineDetectorCancel()
		offlineDetectorCancel = nil
	}

	if !Conf.EnableOfflineHistory {
		return
	}

	// 只在首次启动时初始化运行态并记录启动时间（用于宽限期）
	if offlineDetectorStartTime.IsZero() {
		InitServerRuntimes()
		offlineDetectorStartTime = time.Now()
	}

	ctx, cancel := context.WithCancel(context.Background())
	offlineDetectorCancel = cancel
	offlineDetectorReload = make(chan struct{}, 1)
	go offlineDetectorLoop(ctx)
}

// ReloadOfflineDetectorConfig 在不重启检测循环的前提下，让检测间隔等配置变更即时生效。
// 若循环尚未启动则不做任何事（StartOfflineDetector 会处理启用场景）。
func ReloadOfflineDetectorConfig() {
	offlineDetectorMu.Lock()
	ch := offlineDetectorReload
	offlineDetectorMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default: // 已有待处理的重载信号，避免重复唤醒
	}
}

// InitServerRuntimes 为所有已存在但尚无运行态记录的服务器创建初始运行态。
// 仅创建骨架行（Status=unknown，不预填任何时间戳），保留“是否上报过”的真实判定：
// LastSeenAt 只在 Agent 真实上报时才会被写入。否则此处伪造的 LastSeenAt 会把
// 从未连接 Agent 的服务器误判为“已上报”，导致前台可用性错误显示 100%（应为空值）。
// 离线检测在 DetectOfflineServers 中已对 LastSeenAt==nil 做了跳过处理，不受影响。
func InitServerRuntimes() {
	ServerLock.RLock()
	serverIDs := make([]uint64, 0, len(ServerList))
	for id := range ServerList {
		serverIDs = append(serverIDs, id)
	}
	ServerLock.RUnlock()

	for _, id := range serverIDs {
		var rt model.ServerRuntime
		err := DB.Where("server_id = ?", id).First(&rt).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("NEZHA>> 初始化运行态读取失败 server_id=%d: %v", id, err)
			continue
		}
		// 仅创建骨架行，不预填时间戳：保留 LastSeenAt 为 nil 直到 Agent 真实上报，
		// 从而让可用性判定能区分“从未上报（空值）”与“已上报（真实可用率）”。
		rt = model.ServerRuntime{
			ServerID: id,
			Status:   model.ServerRuntimeStatusUnknown,
		}
		if err := DB.Create(&rt).Error; err != nil {
			log.Printf("NEZHA>> 初始化运行态创建失败 server_id=%d: %v", id, err)
		}
	}
}

func offlineDetectorLoop(ctx context.Context) {
	for {
		interval := time.Duration(Conf.OfflineCheckIntervalSeconds) * time.Second
		if interval < time.Second*5 {
			interval = time.Second * 5
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			DetectOfflineServers()
		case <-offlineDetectorReload:
			// 配置变更（如检测间隔），丢弃未到期的 timer，立即进入下一轮重新读取配置
			if !timer.Stop() {
				<-timer.C
			}
		}
	}
}

// DetectOfflineServers 扫描所有在线服务器，将超过离线阈值未上报的服务器标记为离线。
func DetectOfflineServers() {
	if !Conf.EnableOfflineHistory {
		return
	}
	threshold := time.Duration(Conf.OfflineThresholdSeconds) * time.Second
	if threshold < time.Second*10 {
		threshold = time.Second * 10
	}
	interval := time.Duration(Conf.OfflineCheckIntervalSeconds) * time.Second
	if interval < time.Second*5 {
		interval = time.Second * 5
	}

	now := time.Now()
	// Dashboard 启动后的宽限期，避免重启瞬间误判所有服务器离线
	if now.Sub(offlineDetectorStartTime) <= interval*2 {
		return
	}

	deadline := now.Add(-threshold)
	var runtimes []model.ServerRuntime
	if err := DB.Where("status = ? AND last_seen_at < ?", model.ServerRuntimeStatusOnline, deadline).Find(&runtimes).Error; err != nil {
		log.Printf("NEZHA>> 离线检测查询失败: %v", err)
		return
	}

	for i := range runtimes {
		rt := &runtimes[i]
		if rt.LastSeenAt == nil || rt.CurrentOfflineID != 0 {
			continue
		}
		createOfflineHistory(rt, now, threshold)
	}
}

func createOfflineHistory(rt *model.ServerRuntime, now time.Time, threshold time.Duration) {
	startedAt := rt.LastSeenAt.Add(threshold)

	tx := DB.Begin()
	// 在事务内重新读取运行态，避免并发上报已经关闭离线事件后仍重复创建
	var current model.ServerRuntime
	if err := tx.First(&current, rt.ServerID).Error; err != nil {
		tx.Rollback()
		return
	}
	if current.Status != model.ServerRuntimeStatusOnline || current.CurrentOfflineID != 0 || current.LastSeenAt == nil {
		tx.Rollback()
		return
	}
	if now.Sub(*current.LastSeenAt) <= threshold {
		tx.Rollback()
		return
	}

	history := model.ServerOfflineHistory{
		ServerID:         current.ServerID,
		StartedAt:        startedAt,
		DetectedAt:       now,
		Status:           model.OfflineHistoryStatusOpen,
		Reason:           model.OfflineReasonUnknown,
		ThresholdSeconds: uint64(threshold.Seconds()),
		LastSeenAt:       *current.LastSeenAt,
		LastBootTime:     current.LastBootTime,
		LastUptime:       current.LastUptime,
		LastIP:           current.LastIP,
	}
	if err := tx.Create(&history).Error; err != nil {
		tx.Rollback()
		log.Printf("NEZHA>> 创建离线历史失败: %v", err)
		return
	}

	current.Status = model.ServerRuntimeStatusOffline
	current.LastOfflineAt = &now
	current.CurrentOfflineID = history.ID
	if err := tx.Save(&current).Error; err != nil {
		tx.Rollback()
		log.Printf("NEZHA>> 更新运行态为离线失败: %v", err)
		return
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("NEZHA>> 提交离线历史事务失败: %v", err)
		return
	}

	if Conf.EnableOfflineNotification {
		sendOfflineNotification(current.ServerID, &history)
	}
}

// CloseOfflineHistory 在服务器恢复上报时关闭当前未关闭的离线记录。
func CloseOfflineHistory(rt *model.ServerRuntime, state *model.HostState, host *model.Host, now time.Time) {
	if rt == nil || rt.CurrentOfflineID == 0 {
		return
	}

	var history model.ServerOfflineHistory
	if err := DB.First(&history, rt.CurrentOfflineID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			rt.CurrentOfflineID = 0
			rt.Status = model.ServerRuntimeStatusOnline
			DB.Save(rt)
		}
		return
	}

	if history.Status == model.OfflineHistoryStatusClosed {
		rt.CurrentOfflineID = 0
		rt.Status = model.ServerRuntimeStatusOnline
		DB.Save(rt)
		return
	}

	recoveredBootTime := rt.LastBootTime
	recoveredUptime := rt.LastUptime
	recoveredIP := rt.LastIP
	if host != nil {
		recoveredBootTime = host.BootTime
		recoveredIP = host.IP
	}
	if state != nil {
		recoveredUptime = state.Uptime
	}

	reason := DetectOfflineReason(history.LastBootTime, recoveredBootTime, history.LastUptime, recoveredUptime)
	duration := uint64(now.Sub(history.StartedAt).Seconds())
	if history.StartedAt.After(now) {
		duration = 0
	}

	history.EndedAt = &now
	history.RecoveredAt = &now
	history.DurationSeconds = duration
	history.Reason = reason
	history.Status = model.OfflineHistoryStatusClosed
	history.RecoveredBootTime = recoveredBootTime
	history.RecoveredUptime = recoveredUptime
	history.RecoveredIP = recoveredIP

	// 在事务内更新历史与运行态，避免合并时出现中间态
	if err := func() error {
		tx := DB.Begin()
		if err := tx.Save(&history).Error; err != nil {
			tx.Rollback()
			return err
		}
		rt.Status = model.ServerRuntimeStatusOnline
		rt.CurrentOfflineID = 0
		rt.LastOnlineAt = &now
		if err := tx.Save(rt).Error; err != nil {
			tx.Rollback()
			return err
		}
		return tx.Commit().Error
	}(); err != nil {
		log.Printf("NEZHA>> 关闭离线历史失败: %v", err)
		return
	}

	if Conf.EnableRecoveryNotification {
		// 先合并再发通知，使通知内容（时长/原因/恢复点）与最终落库历史一致
		finalHistory := tryMergeWithPrevious(rt.ServerID, &history)
		sendRecoveryNotification(rt.ServerID, finalHistory)
	} else {
		tryMergeWithPrevious(rt.ServerID, &history)
	}
}

// tryMergeWithPrevious 在服务器恢复后，尝试把当前刚关闭的离线记录并入上一条已关闭记录。
// 当两次离线之间的“真实在线时间” <= OfflineMergeGapSeconds 时，删除当前记录并把上一条延展到本次恢复点，
// 使可用性统计把短时抖动算作一次连续离线。该值在配置加载时已归一化（默认 10，范围 1~3600）。
//
// 返回最终生效的离线记录指针：合并成功时为延展后的上一条记录，未合并或合并失败时为当前记录。
// 调用方据此发送与最终落库一致的恢复通知。
//
// 注意：在线时间用 current.LastSeenAt（最后一次成功上报时间）而非 StartedAt（= LastSeenAt + 离线阈值）
// 来计算，否则会把离线阈值也算进在线窗口，导致本应合并的短抖动被错误保留。
func tryMergeWithPrevious(serverID uint64, current *model.ServerOfflineHistory) *model.ServerOfflineHistory {
	if Conf.OfflineMergeGapSeconds == 0 {
		return current // 防御性：配置未正确归一化时不合并
	}

	var prev model.ServerOfflineHistory
	err := DB.Where("server_id = ? AND status = ? AND id < ?",
		serverID, model.OfflineHistoryStatusClosed, current.ID,
	).Order("id DESC").First(&prev).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("NEZHA>> 查询上一条离线历史失败: %v", err)
		}
		return current
	}
	if prev.EndedAt == nil {
		return current
	}

	// 两次离线之间的真实在线时间 = 本次最后一次上报时间 - 上一条的恢复时间
	gapDuration := time.Duration(Conf.OfflineMergeGapSeconds) * time.Second
	if current.LastSeenAt.Sub(*prev.EndedAt) > gapDuration {
		return current // 在线时间超过阈值，不合并
	}

	// 合并：把 prev 延展到本次恢复点，删除当前记录
	prev.EndedAt = current.EndedAt
	prev.RecoveredAt = current.RecoveredAt
	prev.RecoveredBootTime = current.RecoveredBootTime
	prev.RecoveredUptime = current.RecoveredUptime
	prev.RecoveredIP = current.RecoveredIP
	if prev.StartedAt.Before(*current.EndedAt) {
		prev.DurationSeconds = uint64(current.EndedAt.Sub(prev.StartedAt).Seconds())
	} else {
		prev.DurationSeconds = 0
	}
	// 两段原因不同时降级为 unknown，避免误导
	if prev.Reason != current.Reason {
		prev.Reason = model.OfflineReasonUnknown
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Delete(&model.ServerOfflineHistory{}, current.ID).Error; err != nil {
			return err
		}
		return tx.Save(&prev).Error
	}); err != nil {
		log.Printf("NEZHA>> 合并离线历史失败: %v", err)
		return current // 合并失败，通知仍按当前记录发送
	}

	return &prev
}

// DetectOfflineReason 根据离线前后的 BootTime 与 Uptime 判断离线原因。
func DetectOfflineReason(lastBootTime, recoveredBootTime, lastUptime, recoveredUptime uint64) string {
	if lastBootTime == 0 || recoveredBootTime == 0 || lastUptime == 0 || recoveredUptime == 0 {
		return model.OfflineReasonUnknown
	}
	if recoveredBootTime < lastBootTime {
		return model.OfflineReasonUnknown
	}
	if recoveredBootTime > lastBootTime || recoveredUptime < lastUptime {
		return model.OfflineReasonMachineReboot
	}
	return model.OfflineReasonNetworkDisconnect
}

// GetOrCreateServerRuntime 根据服务器 ID 获取或创建运行态记录。
func GetOrCreateServerRuntime(serverID uint64) (*model.ServerRuntime, error) {
	var rt model.ServerRuntime
	err := DB.Where("server_id = ?", serverID).First(&rt).Error
	if err == nil {
		return &rt, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	rt = model.ServerRuntime{
		ServerID: serverID,
		Status:   model.ServerRuntimeStatusUnknown,
	}
	if err := DB.Create(&rt).Error; err != nil {
		// 可能并发创建，再次尝试读取
		if err := DB.Where("server_id = ?", serverID).First(&rt).Error; err != nil {
			return nil, err
		}
	}
	return &rt, nil
}

// UpdateServerRuntimeOnStateReport 在收到 ReportSystemState 时更新运行态。
// 注意：上报时间戳/状态的写入不依赖 EnableOfflineHistory——可用性（前台展示）需要据此
// 判断“是否上报过”，与离线历史是两个独立开关。仅“关闭离线记录”这一历史相关逻辑
// 仍受 EnableOfflineHistory 控制。
func UpdateServerRuntimeOnStateReport(serverID uint64, state model.HostState) {
	rt, err := GetOrCreateServerRuntime(serverID)
	if err != nil {
		log.Printf("NEZHA>> 获取服务器运行态失败: %v", err)
		return
	}
	now := time.Now()
	if rt.FirstSeenAt == nil {
		rt.FirstSeenAt = &now
	}
	if Conf.EnableOfflineHistory && rt.Status == model.ServerRuntimeStatusOffline {
		CloseOfflineHistory(rt, &state, nil, now)
	}
	rt.Status = model.ServerRuntimeStatusOnline
	rt.LastSeenAt = &now
	rt.LastOnlineAt = &now
	rt.LastUptime = state.Uptime
	if err := DB.Save(rt).Error; err != nil {
		log.Printf("NEZHA>> 更新服务器运行态失败: %v", err)
	}
}

// UpdateServerRuntimeOnHostReport 在收到 ReportSystemInfo 时更新运行态。
// 同 UpdateServerRuntimeOnStateReport：上报时间戳/状态始终写入，仅离线历史逻辑受
// EnableOfflineHistory 控制。
func UpdateServerRuntimeOnHostReport(serverID uint64, host model.Host) {
	rt, err := GetOrCreateServerRuntime(serverID)
	if err != nil {
		log.Printf("NEZHA>> 获取服务器运行态失败: %v", err)
		return
	}
	now := time.Now()
	if rt.FirstSeenAt == nil {
		rt.FirstSeenAt = &now
	}
	if Conf.EnableOfflineHistory && rt.Status == model.ServerRuntimeStatusOffline {
		CloseOfflineHistory(rt, nil, &host, now)
	}
	rt.Status = model.ServerRuntimeStatusOnline
	rt.LastSeenAt = &now
	rt.LastOnlineAt = &now
	rt.LastBootTime = host.BootTime
	rt.LastIP = host.IP
	rt.LastAgentVersion = host.Version
	if err := DB.Save(rt).Error; err != nil {
		log.Printf("NEZHA>> 更新服务器运行态失败: %v", err)
	}
}

// CleanOfflineHistory 清理超过保留天数的离线历史。
func CleanOfflineHistory() {
	if Conf.OfflineHistoryRetentionDays == 0 {
		return
	}
	before := time.Now().AddDate(0, 0, -int(Conf.OfflineHistoryRetentionDays))
	res := DB.Unscoped().Delete(&model.ServerOfflineHistory{}, "started_at < ?", before)
	log.Printf("NEZHA>> Cron 离线历史清理 %d 条, 错误: %v", res.RowsAffected, res.Error)
}

func sendOfflineNotification(serverID uint64, history *model.ServerOfflineHistory) {
	// 只短暂持有读锁复制 server 上下文，避免在 SendNotification 期间持有锁
	var serverCopy model.Server
	ServerLock.RLock()
	server := ServerList[serverID]
	if server != nil {
		serverCopy.ID = server.ID
		serverCopy.Name = server.Name
		serverCopy.Secret = server.Secret
		if server.Host != nil {
			h := *server.Host
			serverCopy.Host = &h
		} else {
			serverCopy.Host = &model.Host{}
		}
		if server.State != nil {
			st := *server.State
			serverCopy.State = &st
		} else {
			serverCopy.State = &model.HostState{}
		}
	}
	ServerLock.RUnlock()
	if serverCopy.ID == 0 {
		serverCopy.ID = serverID
		serverCopy.Name = fmt.Sprintf("Server-%d", serverID)
		serverCopy.Host = &model.Host{}
		serverCopy.State = &model.HostState{}
	}
	lastSeen := history.LastSeenAt.In(Loc).Format("01/02/2006 15:04:05")
	detected := history.DetectedAt.In(Loc).Format("01/02/2006 15:04:05")
	msg := fmt.Sprintf("[离线] %s\n最后上报：%s\n判定离线：%s\n离线阈值：%d 秒\nIP：%s",
		serverCopy.Name, lastSeen, detected, history.ThresholdSeconds, IPDesensitize(history.LastIP))
	SendNotification("default", msg, nil, &serverCopy)
}

func sendRecoveryNotification(serverID uint64, history *model.ServerOfflineHistory) {
	// 只短暂持有读锁复制 server 上下文，避免在 SendNotification 期间持有锁
	var serverCopy model.Server
	ServerLock.RLock()
	server := ServerList[serverID]
	if server != nil {
		serverCopy.ID = server.ID
		serverCopy.Name = server.Name
		serverCopy.Secret = server.Secret
		if server.Host != nil {
			h := *server.Host
			serverCopy.Host = &h
		} else {
			serverCopy.Host = &model.Host{}
		}
		if server.State != nil {
			st := *server.State
			serverCopy.State = &st
		} else {
			serverCopy.State = &model.HostState{}
		}
	}
	ServerLock.RUnlock()
	if serverCopy.ID == 0 {
		serverCopy.ID = serverID
		serverCopy.Name = fmt.Sprintf("Server-%d", serverID)
		serverCopy.Host = &model.Host{}
		serverCopy.State = &model.HostState{}
	}
	recovered := history.RecoveredAt.In(Loc).Format("01/02/2006 15:04:05")
	duration := time.Duration(history.DurationSeconds) * time.Second
	reasonText := history.Reason
	switch reasonText {
	case model.OfflineReasonMachineReboot:
		reasonText = "机器重启"
	case model.OfflineReasonNetworkDisconnect:
		reasonText = "网络断线 / 上报中断"
	default:
		reasonText = "原因未知"
	}
	msg := fmt.Sprintf("[恢复] %s\n恢复时间：%s\n离线时长：%s\n原因：%s\n离线前 IP：%s\n恢复后 IP：%s",
		serverCopy.Name, recovered, duration, reasonText, IPDesensitize(history.LastIP), IPDesensitize(history.RecoveredIP))
	SendNotification("default", msg, nil, &serverCopy)
}
