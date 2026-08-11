package model

import (
	"fmt"
	"strings"

	"github.com/hi2shark/santaizi-dashboard/pkg/utils"
	"gorm.io/gorm"
)

const (
	ModeAlwaysTrigger  = 0
	ModeOnetimeTrigger = 1
)

type AlertRule struct {
	Common
	Name            string
	RulesRaw        string
	Enable          *bool
	TriggerMode     int `gorm:"default:0"`
	NotificationTag string
	Rules           []Rule `gorm:"-" json:"-"`
}

func (r *AlertRule) BeforeSave(_ *gorm.DB) error {
	data, err := utils.Json.Marshal(r.Rules)
	if err != nil {
		return err
	}
	r.RulesRaw = string(data)
	return nil
}

func (r *AlertRule) AfterFind(_ *gorm.DB) error {
	if r.RulesRaw == "" {
		r.Rules = []Rule{}
		return nil
	}
	return utils.Json.Unmarshal([]byte(r.RulesRaw), &r.Rules)
}

func (r *AlertRule) Enabled() bool { return r.Enable != nil && *r.Enable }

func (r *AlertRule) RulesSummary() string {
	parts := make([]string, 0, len(r.Rules))
	for _, rule := range r.Rules {
		thresholds := make([]string, 0, 2)
		if rule.Min > 0 {
			thresholds = append(thresholds, fmt.Sprintf("min: %.2f", rule.Min))
		}
		if rule.Max > 0 {
			thresholds = append(thresholds, fmt.Sprintf("max: %.2f", rule.Max))
		}
		parts = append(parts, strings.TrimSpace(rule.Type+" "+strings.Join(thresholds, ", ")))
	}
	return strings.Join(parts, "; ")
}

func (r *AlertRule) Snapshot(server *Server, db *gorm.DB) []interface{} {
	point := make([]interface{}, 0, len(r.Rules))
	for index := range r.Rules {
		point = append(point, r.Rules[index].Snapshot(server, db))
	}
	return point
}

func (r *AlertRule) Check(points [][]interface{}) (int, bool) {
	maxNum, count := 0, 0
	for index := range r.Rules {
		num := int(r.Rules[index].Duration) // #nosec G115 -- bounded by the API
		if num < 1 {
			num = 1
		}
		if num > maxNum {
			maxNum = num
		}
		if len(points) < num {
			continue
		}
		total, failed := 0.0, 0.0
		for cursor := len(points) - 1; cursor >= 0 && len(points)-num <= cursor; cursor-- {
			total++
			if index < len(points[cursor]) && points[cursor][index] != nil {
				failed++
			}
		}
		if total > 0 && failed/total > 0.7 {
			count++
		}
	}
	return maxNum, count != len(r.Rules)
}
