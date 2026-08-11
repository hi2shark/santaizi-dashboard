package model

import "time"

const (
	TrafficDirectionInbound  = "inbound"
	TrafficDirectionOutbound = "outbound"
	TrafficDirectionTotal    = "total"

	TrafficModeCumulative = "cumulative"
	TrafficModeRecurring  = "recurring"
)

type TrafficPolicy struct {
	Common
	ServerID        uint64     `gorm:"not null;index:idx_traffic_policy_server"`
	Name            string     `gorm:"size:100;not null"`
	Direction       string     `gorm:"size:16;not null;index"`
	Mode            string     `gorm:"size:16;not null;index"`
	CycleStart      *time.Time `gorm:"index"`
	CycleInterval   uint64     `gorm:"not null;default:1"`
	CycleUnit       string     `gorm:"size:8"`
	QuotaBytes      uint64     `gorm:"not null"`
	WarningPercent  float64    `gorm:"not null;default:80"`
	NotificationTag string     `gorm:"size:100"`
	Enabled         bool       `gorm:"not null;default:true;index"`
}

type TrafficPolicyState struct {
	PolicyID           uint64 `gorm:"primaryKey"`
	WindowStart        int64  `gorm:"not null;index"`
	WindowEnd          int64  `gorm:"not null;index"`
	UsedBytes          uint64 `gorm:"not null"`
	WarningNotifiedAt  int64
	ExceededNotifiedAt int64
	LastEvaluatedAt    int64 `gorm:"not null;index"`
	UpdatedAt          time.Time
}
