package model

import "time"

const (
	ServerRuntimeStatusUnknown = "unknown"
	ServerRuntimeStatusOnline  = "online"
	ServerRuntimeStatusOffline = "offline"
)

// ServerRuntime 用于持久化保存每台服务器的运行时状态，
// 使 Dashboard 重启后仍能恢复对在线/离线状态的判断。
type ServerRuntime struct {
	ServerID uint64 `gorm:"primaryKey"`

	Status string `gorm:"index"`

	FirstSeenAt   *time.Time
	LastSeenAt    *time.Time
	LastOnlineAt  *time.Time
	LastOfflineAt *time.Time

	LastBootTime     uint64
	LastUptime       uint64
	LastIP           string
	LastAgentVersion string

	CurrentOfflineID uint64 `gorm:"index"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
