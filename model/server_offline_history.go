package model

import "time"

const (
	OfflineHistoryStatusOpen   = "open"
	OfflineHistoryStatusClosed = "closed"
)

const (
	OfflineReasonUnknown           = "unknown"
	OfflineReasonNetworkDisconnect = "network_disconnect"
	OfflineReasonMachineReboot     = "machine_reboot"
	OfflineReasonAgentRestart      = "agent_restart"
	OfflineReasonDashboardRestart  = "dashboard_restart"
	OfflineReasonManual            = "manual"
)

// ServerOfflineHistory 保存服务器的离线区间，用于后续可用性统计与故障追溯。
type ServerOfflineHistory struct {
	Common

	ServerID uint64 `gorm:"index"`

	StartedAt  time.Time  `gorm:"index"`
	DetectedAt time.Time  `gorm:"index"`
	EndedAt    *time.Time `gorm:"index"`

	DurationSeconds uint64

	Reason string `gorm:"index"`
	Status string `gorm:"index"`

	ThresholdSeconds uint64

	LastSeenAt   time.Time
	LastBootTime uint64
	LastUptime   uint64
	LastIP       string

	RecoveredAt       *time.Time
	RecoveredBootTime uint64
	RecoveredUptime   uint64
	RecoveredIP       string

	Note string
}
