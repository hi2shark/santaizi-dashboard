package singleton

import (
	"math"
	"sort"
	"time"

	"github.com/naiba/nezha/model"
)

// FormatAvailabilityPercent 将可用率按“保留两位小数并向下取整”的方式格式化，
// 避免出现少量离线时间后四舍五入显示为 100.00% 的问题。
func FormatAvailabilityPercent(percent float64) float64 {
	if percent <= 0 {
		return 0
	}
	if percent >= 100 {
		return 100
	}
	return math.Floor(percent*100) / 100
}

// SummarizeOfflineIntervals 计算离线记录在统计窗口 [periodStart, periodEnd] 内的并集时长。
// 重叠的离线区间（异常重复记录、合并残留等）只计算一次，避免可用率被重复扣除。
// 返回并集总时长与最长单段连续离线时长（秒）。
func SummarizeOfflineIntervals(histories []model.ServerOfflineHistory, periodStart, periodEnd time.Time) (uint64, uint64) {
	type interval struct{ start, end time.Time }
	intervals := make([]interval, 0, len(histories))
	for _, h := range histories {
		start := h.StartedAt
		if start.Before(periodStart) {
			start = periodStart
		}
		end := periodEnd
		if h.EndedAt != nil && h.EndedAt.Before(end) {
			end = *h.EndedAt
		}
		if !end.After(start) {
			continue
		}
		intervals = append(intervals, interval{start: start, end: end})
	}
	if len(intervals) == 0 {
		return 0, 0
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].start.Before(intervals[j].start) })

	var total, longest uint64
	curStart, curEnd := intervals[0].start, intervals[0].end
	flush := func() {
		d := uint64(curEnd.Sub(curStart).Seconds())
		total += d
		if d > longest {
			longest = d
		}
	}
	for _, iv := range intervals[1:] {
		if iv.start.After(curEnd) {
			// 与当前段不相交，结算当前段并开启新段
			flush()
			curStart, curEnd = iv.start, iv.end
			continue
		}
		if iv.end.After(curEnd) {
			// 与当前段重叠或相接，延展当前段
			curEnd = iv.end
		}
	}
	flush()
	return total, longest
}

// ServerAvailability 服务器可用性聚合摘要（适合前台展示）。
// AvailabilityPercent 为指针：nil 表示该服务器从未上报过数据（不可统计），
// 非 nil 表示真实可用率（已上报且无离线时为 100）。
type ServerAvailability struct {
	ServerID              uint64   `json:"server_id"`
	Days                  int      `json:"days"`
	OfflineCount          int      `json:"offline_count"`
	TotalOfflineSeconds   uint64   `json:"total_offline_seconds"`
	LongestOfflineSeconds uint64   `json:"longest_offline_seconds"`
	AvailabilityPercent   *float64 `json:"availability_percent"`
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

	// 查询各服务器运行态，用 LastSeenAt 判断是否上报过数据；
	// 从未上报的服务器可用性应为空值（nil），而非 100%。
	// 注意选用 LastSeenAt 而非 FirstSeenAt：InitServerRuntimes / GetOrCreateServerRuntime
	// 在创建运行态时均不写 FirstSeenAt（仅在首次上报时补写），而 LastSeenAt 在每次上报、
	// 以及运行态初始化时都会被写入，是兼容已有数据的可靠“是否上报过”信号。
	var runtimes []model.ServerRuntime
	if err := DB.Select("server_id", "last_seen_at").Where("server_id IN ?", serverIDs).Find(&runtimes).Error; err != nil {
		return nil, 0, err
	}
	reported := make(map[uint64]bool, len(serverIDs))
	for _, rt := range runtimes {
		if rt.LastSeenAt != nil {
			reported[rt.ServerID] = true
		}
	}

	hundred := 100.0
	for _, serverID := range serverIDs {
		result[serverID] = &ServerAvailability{
			ServerID: serverID,
			Days:     days,
		}
		if reported[serverID] {
			result[serverID].AvailabilityPercent = &hundred
		}
	}

	// 离线时长按区间并集计算：重叠的离线记录只计一次，避免可用率被重复扣除
	grouped := make(map[uint64][]model.ServerOfflineHistory, len(serverIDs))
	for _, h := range histories {
		item, ok := result[h.ServerID]
		if !ok {
			continue
		}
		item.OfflineCount++
		grouped[h.ServerID] = append(grouped[h.ServerID], h)
	}
	for serverID, hs := range grouped {
		item := result[serverID]
		item.TotalOfflineSeconds, item.LongestOfflineSeconds = SummarizeOfflineIntervals(hs, periodStart, periodEnd)
	}

	for _, item := range result {
		// 从未上报的服务器保持 nil（不可统计）
		if item.AvailabilityPercent == nil {
			continue
		}
		if periodSeconds == 0 {
			continue
		}
		if item.TotalOfflineSeconds >= periodSeconds {
			zero := 0.0
			item.AvailabilityPercent = &zero
		} else {
			pct := FormatAvailabilityPercent((1.0 - float64(item.TotalOfflineSeconds)/float64(periodSeconds)) * 100)
			item.AvailabilityPercent = &pct
		}
	}

	return result, periodSeconds, nil
}
