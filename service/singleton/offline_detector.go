package singleton

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/hi2shark/santaizi-dashboard/model"
)

var (
	offlineDetectorStartTime time.Time
	offlineDetectorMu        sync.Mutex
	offlineDetectorCancel    context.CancelFunc
	// offlineDetectorReload 用于在不重启整个检测循环的前提下，让检测间隔等配置变更即时生效。
	offlineDetectorReload chan struct{}
	// serverRuntimeMu 串行化所有对 server_runtimes 的“读-改-写”操作（Agent 上报、
	// 离线检测、一致性修复、可用性重置）。上报路径与检测器运行在不同 goroutine，
	// 若不加互斥，整行 Save 可能把对方刚提交的离线状态覆盖掉，遗留永不关闭的
	// 离线记录（前台可用性表现为“无限离线”）。SQLite 不支持 SELECT ... FOR UPDATE，
	// 进程内互斥锁是最简单且各数据库通用的一致性手段。
	serverRuntimeMu sync.Mutex
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
	// 启动（或配置变更重启）时先修复一次历史遗留的异常数据（未关闭/重复的离线记录），
	// 避免“无限离线”要等到第一个检测周期才被自愈
	ReconcileOfflineHistories()
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
			log.Printf("SANTAIZI>> 初始化运行态读取失败 server_id=%d: %v", id, err)
			continue
		}
		// 仅创建骨架行，不预填时间戳：保留 LastSeenAt 为 nil 直到 Agent 真实上报，
		// 从而让可用性判定能区分“从未上报（空值）”与“已上报（真实可用率）”。
		rt = model.ServerRuntime{
			ServerID: id,
			Status:   model.ServerRuntimeStatusUnknown,
		}
		if err := DB.Create(&rt).Error; err != nil {
			log.Printf("SANTAIZI>> 初始化运行态创建失败 server_id=%d: %v", id, err)
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

// DetectOfflineServers 扫描服务器并将确认离线的节点写入离线历史。
// V1 仍按主端 LastSeenAt 超时判定；V2 只认 AvailabilityBucket 共识
// （全部已分配且健康的观测点都看不到，并持续达到阈值）。
func DetectOfflineServers() {
	if !Conf.EnableOfflineHistory {
		return
	}
	// 每轮检测前先修复运行态与离线历史的不一致（如并发遗留的未关闭记录），
	// 避免异常数据长期累积导致“无限离线”或重复记录
	ReconcileOfflineHistories()

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
	if err := DB.Where("status = ? AND last_seen_at < ? AND (protocol = '' OR protocol IS NULL OR protocol <> ?)",
		model.ServerRuntimeStatusOnline, deadline, "v2").Find(&runtimes).Error; err != nil {
		log.Printf("SANTAIZI>> 离线检测查询失败: %v", err)
		return
	}

	var batch []offlineNotice
	for i := range runtimes {
		rt := &runtimes[i]
		if rt.LastSeenAt == nil || rt.CurrentOfflineID != 0 {
			continue
		}
		if history := createOfflineHistory(rt, now, threshold); history != nil {
			batch = append(batch, offlineNotice{serverID: rt.ServerID, history: history})
		}
	}
	detectV2OfflineServers(now, threshold, &batch)
	if Conf.EnableOfflineNotification {
		notifyOfflineBatch(batch)
	}
}

type offlineNotice struct {
	serverID uint64
	history  *model.ServerOfflineHistory
}

func createOfflineHistory(rt *model.ServerRuntime, now time.Time, threshold time.Duration) *model.ServerOfflineHistory {
	serverRuntimeMu.Lock()
	history := createOfflineHistoryTx(rt, now, threshold)
	serverRuntimeMu.Unlock()
	return history
}

// createOfflineHistoryTx 在 serverRuntimeMu 保护下，于单个事务内创建离线记录并把运行态置为离线。
// 返回创建的记录；校验失败（运行态已变化）或写入失败时返回 nil。
func createOfflineHistoryTx(rt *model.ServerRuntime, now time.Time, threshold time.Duration) *model.ServerOfflineHistory {
	startedAt := rt.LastSeenAt.Add(threshold)

	tx := DB.Begin()
	// 在事务内重新读取运行态，避免并发上报已经关闭离线事件后仍重复创建
	var current model.ServerRuntime
	if err := tx.First(&current, rt.ServerID).Error; err != nil {
		tx.Rollback()
		return nil
	}
	if current.Status != model.ServerRuntimeStatusOnline || current.CurrentOfflineID != 0 || current.LastSeenAt == nil {
		tx.Rollback()
		return nil
	}
	if now.Sub(*current.LastSeenAt) <= threshold {
		tx.Rollback()
		return nil
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
		log.Printf("SANTAIZI>> 创建离线历史失败: %v", err)
		return nil
	}

	current.Status = model.ServerRuntimeStatusOffline
	current.LastOfflineAt = &now
	current.CurrentOfflineID = history.ID
	if err := tx.Save(&current).Error; err != nil {
		tx.Rollback()
		log.Printf("SANTAIZI>> 更新运行态为离线失败: %v", err)
		return nil
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("SANTAIZI>> 提交离线历史事务失败: %v", err)
		return nil
	}
	return &history
}

// CloseOfflineHistory 在服务器恢复上报时关闭当前未关闭的离线记录。
func CloseOfflineHistory(rt *model.ServerRuntime, state *model.HostState, host *model.Host, now time.Time) {
	if rt == nil || rt.CurrentOfflineID == 0 {
		return
	}
	serverID := rt.ServerID
	var closed *model.ServerOfflineHistory

	serverRuntimeMu.Lock()
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 事务内重新读取运行态，避免基于过期副本操作
		var current model.ServerRuntime
		if err := tx.First(&current, serverID).Error; err != nil {
			return err
		}
		h, err := closeOfflineHistoryTx(tx, &current, state, host, now)
		if err != nil {
			return err
		}
		closed = h
		return tx.Save(&current).Error
	})
	serverRuntimeMu.Unlock()

	if err != nil {
		log.Printf("SANTAIZI>> 关闭离线历史失败: %v", err)
		return
	}
	// 同步调用方持有的运行态副本，使其后续整行保存不会回退离线字段
	rt.Status = model.ServerRuntimeStatusOnline
	rt.CurrentOfflineID = 0
	if closed != nil {
		rt.LastOnlineAt = &now
		afterOfflineHistoryClosed(serverID, closed)
	}
}

// closeOfflineHistoryTx 在事务内关闭 rt.CurrentOfflineID 指向的离线记录，
// rt 的离线相关字段在事务内一并更新（由调用方负责保存 rt）。
// 返回被关闭的记录；记录不存在或已关闭时仅修正 rt 字段并返回 nil。
func closeOfflineHistoryTx(tx *gorm.DB, rt *model.ServerRuntime, state *model.HostState, host *model.Host, now time.Time) (*model.ServerOfflineHistory, error) {
	if rt.CurrentOfflineID == 0 {
		return nil, nil
	}

	var history model.ServerOfflineHistory
	if err := tx.First(&history, rt.CurrentOfflineID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			rt.CurrentOfflineID = 0
			rt.Status = model.ServerRuntimeStatusOnline
			return nil, nil
		}
		return nil, err
	}

	if history.Status == model.OfflineHistoryStatusClosed {
		rt.CurrentOfflineID = 0
		rt.Status = model.ServerRuntimeStatusOnline
		return nil, nil
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
	duration := durationSecondsBetween(history.StartedAt, now)

	history.EndedAt = &now
	history.RecoveredAt = &now
	history.DurationSeconds = duration
	history.Reason = reason
	history.Status = model.OfflineHistoryStatusClosed
	history.RecoveredBootTime = recoveredBootTime
	history.RecoveredUptime = recoveredUptime
	history.RecoveredIP = recoveredIP

	if err := tx.Save(&history).Error; err != nil {
		return nil, err
	}
	rt.Status = model.ServerRuntimeStatusOnline
	rt.CurrentOfflineID = 0
	rt.LastOnlineAt = &now
	return &history, nil
}

// afterOfflineHistoryClosed 离线记录关闭后的收尾：短抖动合并与恢复通知。
// 先合并再发通知，使通知内容（时长/原因/恢复点）与最终落库历史一致。
func afterOfflineHistoryClosed(serverID uint64, history *model.ServerOfflineHistory) {
	if Conf.EnableRecoveryNotification {
		finalHistory := tryMergeWithPrevious(serverID, history)
		sendRecoveryNotification(serverID, finalHistory)
		return
	}
	tryMergeWithPrevious(serverID, history)
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
			log.Printf("SANTAIZI>> 查询上一条离线历史失败: %v", err)
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
	prev.DurationSeconds = durationSecondsBetween(prev.StartedAt, *current.EndedAt)
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
		log.Printf("SANTAIZI>> 合并离线历史失败: %v", err)
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

// ReconcileOfflineHistories 修复运行态与离线历史不一致的异常数据：
//   - 服务器实际在线（阈值内有上报）但存在未关闭的离线记录（并发覆盖等遗留的孤儿记录）
//     → 按最后上报时间静默关闭，避免可用性被“无限离线”持续扣除；
//   - 服务器确实离线但运行态未指向任何记录，或存在多条未关闭记录（重复创建）
//     → 只保留最早一条并让运行态重新指向它，避免重复计时。
//
// 在离线检测循环每轮执行前调用，异常数据至多一个检测周期即可自愈。
func ReconcileOfflineHistories() {
	if !Conf.EnableOfflineHistory {
		return
	}
	threshold := time.Duration(Conf.OfflineThresholdSeconds) * time.Second
	if threshold < time.Second*10 {
		threshold = time.Second * 10
	}
	now := time.Now()

	var serverIDs []uint64
	if err := DB.Model(&model.ServerOfflineHistory{}).
		Where("status = ?", model.OfflineHistoryStatusOpen).
		Distinct().Pluck("server_id", &serverIDs).Error; err != nil {
		log.Printf("SANTAIZI>> 离线历史一致性检查查询失败: %v", err)
		return
	}
	for _, serverID := range serverIDs {
		reconcileServerOfflineHistories(serverID, now, threshold)
	}
}

func reconcileServerOfflineHistories(serverID uint64, now time.Time, threshold time.Duration) {
	serverRuntimeMu.Lock()
	defer serverRuntimeMu.Unlock()

	var opens []model.ServerOfflineHistory
	if err := DB.Where("server_id = ? AND status = ?", serverID, model.OfflineHistoryStatusOpen).
		Order("id").Find(&opens).Error; err != nil {
		log.Printf("SANTAIZI>> 离线历史一致性检查读取失败 server_id=%d: %v", serverID, err)
		return
	}
	if len(opens) == 0 {
		return
	}

	rt, err := GetOrCreateServerRuntime(serverID)
	if err != nil {
		log.Printf("SANTAIZI>> 离线历史一致性检查读取运行态失败 server_id=%d: %v", serverID, err)
		return
	}
	reporting := rt.LastSeenAt != nil && now.Sub(*rt.LastSeenAt) <= threshold

	if reporting {
		// 服务器正在正常上报：所有未关闭记录都是异常遗留，按最后上报时间关闭。
		// 静默处理（不发恢复通知、不做短抖动合并），仅修复数据。
		closeTime := *rt.LastSeenAt
		err := DB.Transaction(func(tx *gorm.DB) error {
			for i := range opens {
				h := opens[i]
				end := closeTime
				if end.Before(h.StartedAt) {
					end = h.StartedAt
				}
				h.EndedAt = &end
				h.RecoveredAt = &end
				h.DurationSeconds = durationSecondsBetween(h.StartedAt, end)
				h.Status = model.OfflineHistoryStatusClosed
				h.Reason = DetectOfflineReason(h.LastBootTime, rt.LastBootTime, h.LastUptime, rt.LastUptime)
				h.RecoveredBootTime = rt.LastBootTime
				h.RecoveredUptime = rt.LastUptime
				h.RecoveredIP = rt.LastIP
				h.Note = "auto_reconcile"
				if err := tx.Save(&h).Error; err != nil {
					return err
				}
			}
			rt.Status = model.ServerRuntimeStatusOnline
			rt.CurrentOfflineID = 0
			return tx.Save(rt).Error
		})
		if err != nil {
			log.Printf("SANTAIZI>> 修复未关闭离线记录失败 server_id=%d: %v", serverID, err)
		} else {
			log.Printf("SANTAIZI>> 已自动修复 server_id=%d 的 %d 条未关闭离线记录", serverID, len(opens))
		}
		return
	}

	// 服务器确实处于离线状态：只保留最早一条未关闭记录，删除其余的重复记录，
	// 并让运行态重新指向它（后续恢复上报时按正常流程关闭）。
	keep := opens[0]
	err = DB.Transaction(func(tx *gorm.DB) error {
		if len(opens) > 1 {
			ids := make([]uint64, 0, len(opens)-1)
			for _, h := range opens[1:] {
				ids = append(ids, h.ID)
			}
			if err := tx.Unscoped().Delete(&model.ServerOfflineHistory{}, "id IN ?", ids).Error; err != nil {
				return err
			}
		}
		if rt.LastSeenAt == nil {
			return nil // 从未上报的服务器保持 unknown 运行态，仅清理重复记录
		}
		rt.Status = model.ServerRuntimeStatusOffline
		rt.CurrentOfflineID = keep.ID
		if rt.LastOfflineAt == nil {
			detectedAt := keep.DetectedAt
			rt.LastOfflineAt = &detectedAt
		}
		return tx.Save(rt).Error
	})
	if err != nil {
		log.Printf("SANTAIZI>> 修复重复离线记录失败 server_id=%d: %v", serverID, err)
	} else if len(opens) > 1 {
		log.Printf("SANTAIZI>> 已合并 server_id=%d 的 %d 条重复未关闭离线记录", serverID, len(opens))
	}
}

// ResetServerAvailability 重置单台服务器的可用性数据：清空全部离线历史并复位运行态，
// 用于修复异常数据（如遗留未关闭记录导致的“无限离线”）或人工重新统计。
// 返回删除的离线历史条数。
//
// 注意：上报时间会前移到重置时刻。若服务器当前确实离线，检测器会在阈值后新建一条
// 从重置后开始计时的离线记录，而不会按旧的 LastSeenAt 把重置前的时段重新扣除。
func ResetServerAvailability(serverID uint64) (int64, error) {
	serverRuntimeMu.Lock()
	defer serverRuntimeMu.Unlock()

	var deleted int64
	now := time.Now()
	err := DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Unscoped().Delete(&model.ServerOfflineHistory{}, "server_id = ?", serverID)
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected

		var rt model.ServerRuntime
		if err := tx.Where("server_id = ?", serverID).First(&rt).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // 无运行态（从未上报），仅清空历史即可
			}
			return err
		}
		rt.CurrentOfflineID = 0
		if rt.LastSeenAt != nil {
			rt.LastSeenAt = &now
			rt.Status = model.ServerRuntimeStatusOnline
		} else {
			rt.Status = model.ServerRuntimeStatusUnknown // 从未上报：保持未上报判定
		}
		return tx.Save(&rt).Error
	})
	return deleted, err
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

// getOrCreateServerRuntimeTx 在事务内按服务器 ID 获取或创建运行态记录。
func getOrCreateServerRuntimeTx(tx *gorm.DB, serverID uint64) (*model.ServerRuntime, error) {
	var rt model.ServerRuntime
	err := tx.Where("server_id = ?", serverID).First(&rt).Error
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
	if err := tx.Create(&rt).Error; err != nil {
		// 可能并发创建，再次尝试读取
		if err := tx.Where("server_id = ?", serverID).First(&rt).Error; err != nil {
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
	updateServerRuntimeOnReport(serverID, &state, nil)
}

// UpdateServerRuntimeOnHostReport 在收到 ReportSystemInfo 时更新运行态。
// 同 UpdateServerRuntimeOnStateReport：上报时间戳/状态始终写入，仅离线历史逻辑受
// EnableOfflineHistory 控制。
func UpdateServerRuntimeOnHostReport(serverID uint64, host model.Host) {
	updateServerRuntimeOnReport(serverID, nil, &host)
}

// updateServerRuntimeOnReport 在单个事务内完成“读取运行态 → 关闭进行中的离线记录 →
// 写回上报字段”的完整流程，并通过 serverRuntimeMu 与离线检测器互斥。
// 修复的核心竞态：旧实现先读取运行态、事务外修改后整行 Save，若检测器在读写之间
// 提交了离线记录（status=offline, current_offline_id=N），Save 会将其覆盖回 online/0，
// 遗留一条永不关闭的离线记录，前台可用性表现为“无限离线”。
func updateServerRuntimeOnReport(serverID uint64, state *model.HostState, host *model.Host) {
	now := time.Now()
	var closed *model.ServerOfflineHistory

	serverRuntimeMu.Lock()
	err := DB.Transaction(func(tx *gorm.DB) error {
		rt, err := getOrCreateServerRuntimeTx(tx, serverID)
		if err != nil {
			return err
		}
		if rt.FirstSeenAt == nil {
			rt.FirstSeenAt = &now
		}
		// 关闭离线记录必须在写入本次上报字段之前进行：
		// 关闭判定需要“离线前”的 BootTime/Uptime/IP（取自 rt 现有值）
		// 与“恢复后”的值（取自本次上报的 host/state）。
		if Conf.EnableOfflineHistory && rt.Status == model.ServerRuntimeStatusOffline {
			h, err := closeOfflineHistoryTx(tx, rt, state, host, now)
			if err != nil {
				return err
			}
			closed = h
		}
		rt.Status = model.ServerRuntimeStatusOnline
		rt.LastSeenAt = &now
		rt.LastOnlineAt = &now
		if state != nil {
			rt.LastUptime = state.Uptime
		}
		if host != nil {
			rt.LastBootTime = host.BootTime
			rt.LastIP = host.IP
			rt.LastAgentVersion = host.Version
		}
		return tx.Save(rt).Error
	})
	serverRuntimeMu.Unlock()

	if err != nil {
		log.Printf("SANTAIZI>> 更新服务器运行态失败: %v", err)
		return
	}
	if closed != nil {
		afterOfflineHistoryClosed(serverID, closed)
	}
}

// CleanOfflineHistory 清理超过保留天数的离线历史。
func CleanOfflineHistory() {
	if Conf.OfflineHistoryRetentionDays == 0 {
		return
	}
	before := time.Now().AddDate(0, 0, -int(Conf.OfflineHistoryRetentionDays))
	res := DB.Unscoped().Delete(&model.ServerOfflineHistory{}, "started_at < ?", before)
	log.Printf("SANTAIZI>> Cron 离线历史清理 %d 条, 错误: %v", res.RowsAffected, res.Error)
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
	if line := v2ObserverLine(serverID); line != "" {
		msg += "\n" + line
	}
	SendNotification("default", msg, NotificationMuteLabel.ServerOffline(serverID), &serverCopy)
}

func notifyOfflineBatch(batch []offlineNotice) {
	if len(batch) == 0 {
		return
	}
	if len(batch) == 1 {
		sendOfflineNotification(batch[0].serverID, batch[0].history)
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[离线] %d 台主机", len(batch))
	for _, item := range batch {
		name := offlineServerName(item.serverID)
		lastSeen := item.history.LastSeenAt.In(Loc).Format("01/02/2006 15:04:05")
		fmt.Fprintf(&b, "\n%s  最后上报 %s", name, lastSeen)
	}
	SendNotification("default", b.String(), nil)
}

func offlineServerName(serverID uint64) string {
	ServerLock.RLock()
	server := ServerList[serverID]
	ServerLock.RUnlock()
	if server != nil && server.Name != "" {
		return server.Name
	}
	return fmt.Sprintf("Server-%d", serverID)
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
	SendNotification("default", msg, NotificationMuteLabel.ServerRecovery(serverID), &serverCopy)
}

func durationSecondsBetween(start, end time.Time) uint64 {
	if !end.After(start) {
		return 0
	}
	d := end.Sub(start)
	sec := d / time.Second
	if d%time.Second >= time.Second/2 {
		sec++
	}
	return uint64(sec)
}
