package traffic

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/gorm"
)

const (
	hourlyBuckets     = 24
	maxDailyLookback  = 90 * 24 * time.Hour
	maxTimeZoneLength = 64
)

type Point struct {
	Start time.Time
	End   time.Time
	Bytes uint64
}

type Summary struct {
	PolicyID     uint64
	Name         string
	UsedBytes    uint64
	QuotaBytes   uint64
	UsagePercent float64
	Status       string
}

type PolicyHistory struct {
	Policy model.TrafficPolicy
	Usage  Usage
	Hourly []Point
	Daily  []Point
}

func LocationOrUTC(name string) *time.Location {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxTimeZoneLength {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil || loc == nil {
		return time.UTC
	}
	return loc
}

func Summaries(db *gorm.DB, serverIDs []uint64, now time.Time) (map[uint64][]Summary, error) {
	result := make(map[uint64][]Summary, len(serverIDs))
	if len(serverIDs) == 0 {
		return result, nil
	}
	var policies []model.TrafficPolicy
	if err := db.Where("server_id IN ? AND enabled = ?", serverIDs, true).Order("id ASC").Find(&policies).Error; err != nil {
		return nil, err
	}
	for _, policy := range policies {
		usage, err := Calculate(db, policy, now)
		if err != nil {
			return nil, err
		}
		result[policy.ServerID] = append(result[policy.ServerID], Summary{
			PolicyID:     policy.ID,
			Name:         policy.Name,
			UsedBytes:    usage.UsedBytes,
			QuotaBytes:   usage.QuotaBytes,
			UsagePercent: usage.UsagePercent,
			Status:       usage.Status,
		})
	}
	return result, nil
}

func Histories(db *gorm.DB, serverID uint64, now time.Time, loc *time.Location) ([]PolicyHistory, error) {
	if loc == nil {
		loc = time.UTC
	}
	var policies []model.TrafficPolicy
	if err := db.Where("server_id = ?", serverID).Order("id ASC").Find(&policies).Error; err != nil {
		return nil, err
	}
	items := make([]PolicyHistory, 0, len(policies))
	var binding model.ServerNodeBinding
	hasBinding := true
	if err := db.Where("server_id = ? AND current = ?", serverID, true).First(&binding).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		hasBinding = false
	}
	for _, policy := range policies {
		usage, err := Calculate(db, policy, now)
		if err != nil {
			return nil, err
		}
		item := PolicyHistory{Policy: policy, Usage: usage, Hourly: emptyHourly(now), Daily: emptyDaily(usage.WindowStart, now, loc)}
		if hasBinding {
			hourly, err := hourlySeries(db, binding.NodeUUID, policy.Direction, now)
			if err != nil {
				return nil, err
			}
			daily, err := dailySeries(db, binding.NodeUUID, policy.Direction, usage.WindowStart, now, loc)
			if err != nil {
				return nil, err
			}
			item.Hourly = hourly
			item.Daily = daily
		}
		items = append(items, item)
	}
	return items, nil
}

func emptyHourly(now time.Time) []Point {
	currentHour := now.Truncate(time.Hour)
	start := currentHour.Add(-time.Duration(hourlyBuckets-1) * time.Hour)
	points := make([]Point, hourlyBuckets)
	for i := 0; i < hourlyBuckets; i++ {
		bucket := start.Add(time.Duration(i) * time.Hour)
		end := bucket.Add(time.Hour)
		if i == hourlyBuckets-1 {
			end = now
		}
		points[i] = Point{Start: bucket, End: end}
	}
	return points
}

func emptyDaily(windowStart, now time.Time, loc *time.Location) []Point {
	from, to := dailyRange(windowStart, now, loc)
	if to.Before(from) {
		return []Point{}
	}
	var points []Point
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		points = append(points, Point{Start: day, End: day.AddDate(0, 0, 1)})
	}
	return points
}

func dailyRange(windowStart, now time.Time, loc *time.Location) (time.Time, time.Time) {
	localNow := now.In(loc)
	to := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
	lookback := now.Add(-maxDailyLookback)
	start := windowStart
	if start.Before(lookback) {
		start = lookback
	}
	if start.After(now) {
		return to.AddDate(0, 0, 1), to
	}
	localStart := start.In(loc)
	from := time.Date(localStart.Year(), localStart.Month(), localStart.Day(), 0, 0, 0, 0, loc)
	return from, to
}

func hourlySeries(db *gorm.DB, node []byte, direction string, now time.Time) ([]Point, error) {
	points := emptyHourly(now)
	if len(points) == 0 {
		return points, nil
	}
	currentHour := points[len(points)-1].Start
	var rows []model.StateRollup
	if err := db.Where("node_uuid = ? AND resolution = ? AND window_start >= ? AND window_start < ?",
		node, "1h", points[0].Start.UnixNano(), currentHour.UnixNano()).Find(&rows).Error; err != nil {
		return nil, err
	}
	byStart := make(map[int64]uint64, len(rows))
	for _, row := range rows {
		byStart[row.WindowStart] = addBytes(byStart[row.WindowStart], rollupBytes(row, direction))
	}
	for i := 0; i < len(points)-1; i++ {
		points[i].Bytes = byStart[points[i].Start.UnixNano()]
	}
	used, err := sumMinuteUsage(db, node, directionColumn(direction), currentHour, now)
	if err != nil {
		return nil, err
	}
	points[len(points)-1].Bytes = used
	return points, nil
}

func dailySeries(db *gorm.DB, node []byte, direction string, windowStart, now time.Time, loc *time.Location) ([]Point, error) {
	points := emptyDaily(windowStart, now, loc)
	if len(points) == 0 {
		return points, nil
	}
	from := points[0].Start
	currentHour := now.Truncate(time.Hour)
	var rows []model.StateRollup
	if err := db.Where("node_uuid = ? AND resolution = ? AND window_start >= ? AND window_start < ?",
		node, "1h", from.UnixNano(), currentHour.UnixNano()).Find(&rows).Error; err != nil {
		return nil, err
	}
	sums := make(map[string]uint64, len(points))
	for _, row := range rows {
		key := time.Unix(0, row.WindowStart).In(loc).Format("2006-01-02")
		sums[key] = addBytes(sums[key], rollupBytes(row, direction))
	}
	used, err := sumMinuteUsage(db, node, directionColumn(direction), currentHour, now)
	if err != nil {
		return nil, err
	}
	if used > 0 {
		key := now.In(loc).Format("2006-01-02")
		sums[key] = addBytes(sums[key], used)
	}
	for i := range points {
		points[i].Bytes = sums[points[i].Start.In(loc).Format("2006-01-02")]
	}
	return points, nil
}

func directionColumn(direction string) string {
	switch direction {
	case model.TrafficDirectionOutbound:
		return "net_out_total"
	case model.TrafficDirectionTotal:
		return "net_in_total + net_out_total"
	default:
		return "net_in_total"
	}
}

func rollupBytes(row model.StateRollup, direction string) uint64 {
	switch direction {
	case model.TrafficDirectionOutbound:
		return row.NetOutTotal
	case model.TrafficDirectionTotal:
		return addBytes(row.NetInTotal, row.NetOutTotal)
	default:
		return row.NetInTotal
	}
}

func addBytes(left, right uint64) uint64 {
	if math.MaxUint64-left < right {
		return math.MaxUint64
	}
	return left + right
}
