package singleton

import (
	"time"

	"github.com/naiba/nezha/model"
)

// ServerAvailability 服务器可用性聚合摘要（适合前台展示）。
type ServerAvailability struct {
	ServerID              uint64  `json:"server_id"`
	Days                  int     `json:"days"`
	OfflineCount          int     `json:"offline_count"`
	TotalOfflineSeconds   uint64  `json:"total_offline_seconds"`
	LongestOfflineSeconds uint64  `json:"longest_offline_seconds"`
	AvailabilityPercent   float64 `json:"availability_percent"`
}

// GetServerAvailabilitySummaries 批量计算多台服务器在最近 days 天内的可用性摘要。
// 返回 map[server_id]*ServerAvailability，只包含传入的 serverIDs。
func GetServerAvailabilitySummaries(serverIDs []uint64, days int) (map[uint64]*ServerAvailability, uint64, error) {
	result := make(map[uint64]*ServerAvailability, len(serverIDs))
	if len(serverIDs) == 0 || days <= 0 {
		return result, 0, nil
	}

	now := time.Now()
	periodStart := now.AddDate(0, 0, -days)
	periodEnd := now
	periodSeconds := uint64(periodEnd.Sub(periodStart).Seconds())

	var histories []model.ServerOfflineHistory
	if err := DB.Where(
		"server_id IN ? AND started_at < ? AND (ended_at IS NULL OR ended_at > ?)",
		serverIDs, periodEnd, periodStart,
	).Order("started_at DESC").Find(&histories).Error; err != nil {
		return nil, 0, err
	}

	for _, serverID := range serverIDs {
		result[serverID] = &ServerAvailability{
			ServerID:            serverID,
			Days:                days,
			AvailabilityPercent: 100.0,
		}
	}

	for _, h := range histories {
		start := h.StartedAt
		if start.Before(periodStart) {
			start = periodStart
		}
		end := periodEnd
		if h.EndedAt != nil && h.EndedAt.Before(end) {
			end = *h.EndedAt
		}
		if end.Before(start) {
			continue
		}
		duration := uint64(end.Sub(start).Seconds())

		item, ok := result[h.ServerID]
		if !ok {
			continue
		}
		item.OfflineCount++
		item.TotalOfflineSeconds += duration
		if duration > item.LongestOfflineSeconds {
			item.LongestOfflineSeconds = duration
		}
	}

	for _, item := range result {
		if periodSeconds == 0 {
			continue
		}
		if item.TotalOfflineSeconds >= periodSeconds {
			item.AvailabilityPercent = 0.0
		} else {
			item.AvailabilityPercent = (1.0 - float64(item.TotalOfflineSeconds)/float64(periodSeconds)) * 100
		}
	}

	return result, periodSeconds, nil
}
