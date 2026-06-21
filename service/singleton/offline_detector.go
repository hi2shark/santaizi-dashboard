package singleton

import (
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
	offlineDetectorOnce      sync.Once
)

// StartOfflineDetector 启动离线检测任务。
func StartOfflineDetector() {
	if !Conf.EnableOfflineHistory {
		return
	}
	offlineDetectorOnce.Do(func() {
		// 为旧版本升级后的已有服务器初始化运行态，避免重启瞬间误判离线
		InitServerRuntimes()
		offlineDetectorStartTime = time.Now()
		go offlineDetectorLoop()
	})
}

// InitServerRuntimes 为所有已存在但尚无运行态记录的服务器创建初始运行态。
// 升级场景下，这能确保离线检测平滑开始，不会因缺少历史记录而漏判或误报。
func InitServerRuntimes() {
	ServerLock.RLock()
	serverIDs := make([]uint64, 0, len(ServerList))
	for id := range ServerList {
		serverIDs = append(serverIDs, id)
	}
	ServerLock.RUnlock()

	now := time.Now()
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
		rt = model.ServerRuntime{
			ServerID:     id,
			Status:       model.ServerRuntimeStatusOnline,
			LastSeenAt:   &now,
			LastOnlineAt: &now,
		}
		if err := DB.Create(&rt).Error; err != nil {
			log.Printf("NEZHA>> 初始化运行态创建失败 server_id=%d: %v", id, err)
		}
	}
}

func offlineDetectorLoop() {
	interval := time.Duration(Conf.OfflineCheckIntervalSeconds) * time.Second
	if interval < time.Second*5 {
		interval = time.Second * 5
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		DetectOfflineServers()
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

	if err := DB.Save(&history).Error; err != nil {
		log.Printf("NEZHA>> 关闭离线历史失败: %v", err)
		return
	}

	rt.Status = model.ServerRuntimeStatusOnline
	rt.CurrentOfflineID = 0
	rt.LastOnlineAt = &now
	if err := DB.Save(rt).Error; err != nil {
		log.Printf("NEZHA>> 更新运行态为在线失败: %v", err)
		return
	}

	if Conf.EnableRecoveryNotification {
		sendRecoveryNotification(rt.ServerID, &history)
	}
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
func UpdateServerRuntimeOnStateReport(serverID uint64, state model.HostState) {
	if !Conf.EnableOfflineHistory {
		return
	}
	rt, err := GetOrCreateServerRuntime(serverID)
	if err != nil {
		log.Printf("NEZHA>> 获取服务器运行态失败: %v", err)
		return
	}
	now := time.Now()
	if rt.FirstSeenAt == nil {
		rt.FirstSeenAt = &now
	}
	if rt.Status == model.ServerRuntimeStatusOffline {
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
func UpdateServerRuntimeOnHostReport(serverID uint64, host model.Host) {
	if !Conf.EnableOfflineHistory {
		return
	}
	rt, err := GetOrCreateServerRuntime(serverID)
	if err != nil {
		log.Printf("NEZHA>> 获取服务器运行态失败: %v", err)
		return
	}
	now := time.Now()
	if rt.FirstSeenAt == nil {
		rt.FirstSeenAt = &now
	}
	if rt.Status == model.ServerRuntimeStatusOffline {
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
