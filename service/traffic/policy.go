package traffic

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Usage struct {
	PolicyID       uint64
	ServerID       uint64
	Direction      string
	Mode           string
	WindowStart    time.Time
	WindowEnd      *time.Time
	UsedBytes      uint64
	QuotaBytes     uint64
	WarningPercent float64
	UsagePercent   float64
	Status         string
	UpdatedAt      time.Time
}

func Validate(policy *model.TrafficPolicy) error {
	policy.Name = strings.TrimSpace(policy.Name)
	policy.Direction = strings.ToLower(strings.TrimSpace(policy.Direction))
	policy.Mode = strings.ToLower(strings.TrimSpace(policy.Mode))
	policy.CycleUnit = strings.ToLower(strings.TrimSpace(policy.CycleUnit))
	if policy.Name == "" || len(policy.Name) > 100 {
		return errors.New("traffic policy name is required and must not exceed 100 characters")
	}
	if policy.Direction != model.TrafficDirectionInbound && policy.Direction != model.TrafficDirectionOutbound && policy.Direction != model.TrafficDirectionTotal {
		return errors.New("traffic direction must be inbound, outbound, or total")
	}
	if policy.Mode != model.TrafficModeCumulative && policy.Mode != model.TrafficModeRecurring {
		return errors.New("traffic mode must be cumulative or recurring")
	}
	if policy.QuotaBytes == 0 {
		return errors.New("traffic quota must be greater than zero")
	}
	if policy.WarningPercent <= 0 || policy.WarningPercent >= 100 {
		return errors.New("warning percent must be greater than zero and less than 100")
	}
	if policy.Mode == model.TrafficModeRecurring {
		if policy.CycleStart == nil || policy.CycleStart.IsZero() {
			return errors.New("recurring traffic policy requires cycle_start")
		}
		if policy.CycleInterval == 0 {
			return errors.New("recurring traffic policy requires a positive cycle_interval")
		}
		switch policy.CycleUnit {
		case "hour", "day", "week", "month", "year":
		default:
			return errors.New("cycle unit must be hour, day, week, month, or year")
		}
	} else {
		policy.CycleInterval = 1
		policy.CycleUnit = ""
	}
	return nil
}

func ValidateAll(policies []model.TrafficPolicy) error {
	for i := range policies {
		if err := Validate(&policies[i]); err != nil {
			return err
		}
	}
	return nil
}

func Replace(tx *gorm.DB, serverID uint64, policies []model.TrafficPolicy) error {
	var existing []model.TrafficPolicy
	if err := tx.Where("server_id = ?", serverID).Find(&existing).Error; err != nil {
		return err
	}
	keep := make(map[uint64]struct{}, len(policies))
	seen := make(map[uint64]struct{}, len(policies))
	for i := range policies {
		policy := &policies[i]
		policy.ServerID = serverID
		if policy.ID == 0 {
			if err := tx.Create(policy).Error; err != nil {
				return err
			}
			continue
		}
		if _, dup := seen[policy.ID]; dup {
			return fmt.Errorf("duplicate traffic policy id %d", policy.ID)
		}
		seen[policy.ID] = struct{}{}
		var row model.TrafficPolicy
		if err := tx.First(&row, "id = ? AND server_id = ?", policy.ID, serverID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("traffic policy %d does not belong to this server", policy.ID)
			}
			return err
		}
		policy.CreatedAt = row.CreatedAt
		if err := tx.Save(policy).Error; err != nil {
			return err
		}
		keep[policy.ID] = struct{}{}
	}
	for _, row := range existing {
		if _, ok := keep[row.ID]; ok {
			continue
		}
		if err := tx.Unscoped().Delete(&model.TrafficPolicy{}, "id = ? AND server_id = ?", row.ID, serverID).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&model.TrafficPolicyState{}, "policy_id = ?", row.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

func Window(policy model.TrafficPolicy, now time.Time) (time.Time, *time.Time, error) {
	if policy.Mode == model.TrafficModeCumulative {
		start := policy.CreatedAt
		if policy.CycleStart != nil && !policy.CycleStart.IsZero() {
			start = *policy.CycleStart
		}
		if start.IsZero() {
			start = now
		}
		return start, nil, nil
	}
	if err := Validate(&policy); err != nil {
		return time.Time{}, nil, err
	}
	anchor := policy.CycleStart.In(now.Location())
	if now.Before(anchor) {
		end := addCycle(anchor, policy.CycleInterval, policy.CycleUnit)
		return anchor, &end, nil
	}
	start := anchor
	switch policy.CycleUnit {
	case "hour":
		seconds := int64(policy.CycleInterval) * int64(time.Hour/time.Second) // #nosec G115 -- validated API bound
		start = anchor.Add(time.Duration((now.Unix()-anchor.Unix())/seconds*seconds) * time.Second)
	case "day":
		days := int(now.Sub(anchor).Hours() / 24)
		step := int(policy.CycleInterval) // #nosec G115 -- validated API bound
		start = anchor.AddDate(0, 0, days/step*step)
	case "week":
		days := int(now.Sub(anchor).Hours() / 24)
		step := 7 * int(policy.CycleInterval) // #nosec G115 -- validated API bound
		start = anchor.AddDate(0, 0, days/step*step)
	case "month", "year":
		// Calendar periods are deliberately advanced with AddDate so month ends
		// and leap years follow Go's normalized calendar semantics.
		for {
			next := addCycle(start, policy.CycleInterval, policy.CycleUnit)
			if next.After(now) {
				break
			}
			start = next
		}
	}
	end := addCycle(start, policy.CycleInterval, policy.CycleUnit)
	return start, &end, nil
}

func addCycle(value time.Time, interval uint64, unit string) time.Time {
	step := int(interval) // #nosec G115 -- API restricts interval
	switch unit {
	case "year":
		return value.AddDate(step, 0, 0)
	case "month":
		return value.AddDate(0, step, 0)
	case "week":
		return value.AddDate(0, 0, 7*step)
	case "day":
		return value.AddDate(0, 0, step)
	default:
		return value.Add(time.Duration(interval) * time.Hour)
	}
}

func Calculate(db *gorm.DB, policy model.TrafficPolicy, now time.Time) (Usage, error) {
	start, end, err := Window(policy, now)
	if err != nil {
		return Usage{}, err
	}
	var binding model.ServerNodeBinding
	if err := db.Where("server_id = ? AND current = ?", policy.ServerID, true).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return usageResult(policy, start, end, 0, now), nil
		}
		return Usage{}, err
	}
	upper := now
	if end != nil && end.Before(upper) {
		upper = *end
	}
	used, err := sumRollupUsage(db, binding.NodeUUID, policy.Direction, start, upper)
	if err != nil {
		return Usage{}, err
	}
	return usageResult(policy, start, end, used, now), nil
}

func sumRollupUsage(db *gorm.DB, node []byte, direction string, start, end time.Time) (uint64, error) {
	if !end.After(start) {
		return 0, nil
	}
	firstFullHour := start.Truncate(time.Hour)
	if firstFullHour.Before(start) {
		firstFullHour = firstFullHour.Add(time.Hour)
	}
	lastFullHourEnd := end.Truncate(time.Hour)
	column := "net_in_total"
	if direction == model.TrafficDirectionOutbound {
		column = "net_out_total"
	} else if direction == model.TrafficDirectionTotal {
		column = "net_in_total + net_out_total"
	}
	var total uint64
	if firstFullHour.Before(lastFullHourEnd) {
		var hourly uint64
		if err := db.Model(&model.StateRollup{}).Select("COALESCE(SUM("+column+"), 0)").
			Where("node_uuid = ? AND resolution = ? AND window_start >= ? AND window_start < ?", node, "1h", firstFullHour.UnixNano(), lastFullHourEnd.UnixNano()).Scan(&hourly).Error; err != nil {
			return 0, err
		}
		total = hourly
		leading, err := sumMinuteUsage(db, node, column, start, firstFullHour)
		if err != nil {
			return 0, err
		}
		trailing, err := sumMinuteUsage(db, node, column, lastFullHourEnd, end)
		if err != nil {
			return 0, err
		}
		if math.MaxUint64-leading < trailing {
			return math.MaxUint64, nil
		}
		edges := leading + trailing
		if math.MaxUint64-total < edges {
			return math.MaxUint64, nil
		}
		return total + edges, nil
	}
	return sumMinuteUsage(db, node, column, start, end)
}

func sumMinuteUsage(db *gorm.DB, node []byte, column string, start, end time.Time) (uint64, error) {
	if !end.After(start) {
		return 0, nil
	}
	var result uint64
	err := db.Model(&model.StateRollup{}).Select("COALESCE(SUM("+column+"), 0)").
		Where("node_uuid = ? AND resolution = ? AND window_start >= ? AND window_start < ?", node, "1m", start.UnixNano(), end.UnixNano()).Scan(&result).Error
	return result, err
}

func usageResult(policy model.TrafficPolicy, start time.Time, end *time.Time, used uint64, now time.Time) Usage {
	percent := float64(used) * 100 / float64(policy.QuotaBytes)
	status := "normal"
	if percent >= 100 {
		status = "exceeded"
	} else if percent >= policy.WarningPercent {
		status = "warning"
	}
	return Usage{PolicyID: policy.ID, ServerID: policy.ServerID, Direction: policy.Direction, Mode: policy.Mode,
		WindowStart: start, WindowEnd: end, UsedBytes: used, QuotaBytes: policy.QuotaBytes,
		WarningPercent: policy.WarningPercent, UsagePercent: percent, Status: status, UpdatedAt: now}
}

func EvaluateAll(db *gorm.DB, now time.Time, notify func(tag, message string, serverID uint64)) error {
	var policies []model.TrafficPolicy
	if err := db.Where("enabled = ?", true).Find(&policies).Error; err != nil {
		return err
	}
	for _, policy := range policies {
		usage, err := Calculate(db, policy, now)
		if err != nil {
			return err
		}
		var state model.TrafficPolicyState
		_ = db.First(&state, "policy_id = ?", policy.ID).Error
		windowStart := usage.WindowStart.UnixNano()
		windowEnd := int64(0)
		if usage.WindowEnd != nil {
			windowEnd = usage.WindowEnd.UnixNano()
		}
		if state.WindowStart != windowStart || state.WindowEnd != windowEnd {
			state = model.TrafficPolicyState{PolicyID: policy.ID, WindowStart: windowStart, WindowEnd: windowEnd}
		}
		state.UsedBytes = usage.UsedBytes
		state.LastEvaluatedAt = now.UnixNano()
		if notify != nil && usage.Status == "warning" && state.WarningNotifiedAt == 0 {
			notify(policy.NotificationTag, fmt.Sprintf("%s traffic usage reached %.1f%%", policy.Name, usage.UsagePercent), policy.ServerID)
			state.WarningNotifiedAt = now.UnixNano()
		}
		if notify != nil && usage.Status == "exceeded" && state.ExceededNotifiedAt == 0 {
			notify(policy.NotificationTag, fmt.Sprintf("%s traffic quota exceeded", policy.Name), policy.ServerID)
			state.ExceededNotifiedAt = now.UnixNano()
		}
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "policy_id"}}, UpdateAll: true}).Create(&state).Error; err != nil {
			return err
		}
	}
	return nil
}
