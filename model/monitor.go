package model

import (
	"fmt"
	"log"

	"github.com/hi2shark/santaizi-dashboard/pkg/utils"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

const (
	_ = iota
	MonitorTypeHTTPGet
	MonitorTypeICMPPing
	MonitorTypeTCPPing
)

const (
	MonitorCoverAll = iota
	MonitorCoverIgnoreAll
)

type Monitor struct {
	Common
	Name                string
	Type                uint8
	Target              string
	SkipServersRaw      string
	Duration            uint64
	Notify              bool
	NotificationTag     string
	Cover               uint8
	EnableShowInService bool `gorm:"default:false"`
	MinLatency          float32
	MaxLatency          float32
	LatencyNotify       bool

	SkipServers map[uint64]bool `gorm:"-" json:"-"`
	CronJobID   cron.EntryID    `gorm:"-" json:"-"`
}

func (m *Monitor) CronSpec() string {
	if m.Duration == 0 {
		m.Duration = 30
	}
	return fmt.Sprintf("@every %ds", m.Duration)
}

func (m *Monitor) AfterFind(_ *gorm.DB) error {
	if err := m.InitSkipServers(); err != nil {
		log.Println("SANTAIZI>> Monitor.AfterFind:", err)
	}
	return nil
}

func (m *Monitor) InitSkipServers() error {
	var serverIDs []uint64
	if m.SkipServersRaw != "" {
		if err := utils.Json.Unmarshal([]byte(m.SkipServersRaw), &serverIDs); err != nil {
			return err
		}
	}
	m.SkipServers = make(map[uint64]bool, len(serverIDs))
	for _, id := range serverIDs {
		m.SkipServers[id] = true
	}
	return nil
}
