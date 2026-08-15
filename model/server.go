package model

import (
	"log"
	"time"

	"github.com/hi2shark/santaizi-dashboard/pkg/utils"
	"gorm.io/gorm"
)

type Server struct {
	Common
	Name             string
	Tag              string   // 分组名
	Secret           string   `gorm:"-" json:"-"`
	SecretCiphertext []byte   `gorm:"column:secret_ciphertext;type:BLOB;not null;uniqueIndex" json:"-"`
	Note             string   `json:"-"`                    // 管理员可见备注
	PublicNote       string   `json:"PublicNote,omitempty"` // 公开备注
	DisplayIndex     int      // 展示排序，越大越靠前
	HideForGuest     bool     // 对游客隐藏
	EnableDDNS       bool     // 启用DDNS
	DDNSProfiles     []uint64 `gorm:"-" json:"-"` // DDNS配置

	DDNSProfilesRaw      string `gorm:"default:'[]';column:ddns_profiles_raw" json:"-"`
	MonitoringOptionsRaw string `gorm:"default:'{}';column:monitoring_options_raw" json:"-"`
	ProbeTarget          string `gorm:"size:255"`
	ProbeTCPPorts        string `gorm:"size:64"`
	ProbeEnableICMP      *bool  `gorm:"not null;default:1"`
	ProbeEnableTCP       *bool  `gorm:"not null;default:1"`
	ProbeEnableMTR       *bool  `gorm:"not null;default:1"`

	Host       *Host                  `gorm:"-"`
	State      *HostState             `gorm:"-"`
	LastActive time.Time              `gorm:"-"`
	Telemetry  *TelemetryPresentation `gorm:"-" json:"Telemetry,omitempty"`

	PrevTransferInSnapshot  int64 `gorm:"-" json:"-"` // 上次数据点时的入站使用量
	PrevTransferOutSnapshot int64 `gorm:"-" json:"-"` // 上次数据点时的出站使用量
}

type TelemetryPresentation struct {
	Host         string `json:"host"`
	Connectivity string `json:"connectivity"`
	Available    *bool  `json:"available"`
	Coverage     string `json:"coverage"`
}

func (s *Server) CopyFromRunningServer(old *Server) {
	s.Host = old.Host
	s.State = old.State
	s.LastActive = old.LastActive
	s.PrevTransferInSnapshot = old.PrevTransferInSnapshot
	s.PrevTransferOutSnapshot = old.PrevTransferOutSnapshot
}

func (s *Server) AfterFind(tx *gorm.DB) error {
	secret, err := decryptSecret(s.SecretCiphertext)
	if err != nil {
		return err
	}
	s.Secret = secret
	if s.DDNSProfilesRaw != "" {
		if err := utils.Json.Unmarshal([]byte(s.DDNSProfilesRaw), &s.DDNSProfiles); err != nil {
			log.Println("SANTAIZI>> Server.AfterFind:", err)
			return nil
		}
	}
	return nil
}

func (s *Server) BeforeSave(tx *gorm.DB) error {
	value, err := encryptSecret(s.Secret)
	if err != nil {
		return err
	}
	s.SecretCiphertext = value
	return nil
}
