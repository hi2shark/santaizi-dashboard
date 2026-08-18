package traffic

import (
	"bytes"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLocationOrUTC(t *testing.T) {
	if LocationOrUTC("").String() != "UTC" {
		t.Fatal("empty should be UTC")
	}
	if LocationOrUTC("Not/AZone").String() != "UTC" {
		t.Fatal("invalid should be UTC")
	}
	loc := LocationOrUTC("Asia/Shanghai")
	if loc.String() != "Asia/Shanghai" {
		t.Fatalf("loc=%s", loc)
	}
}

func TestHourlySeriesUsesHourRowsAndMinuteCurrentHour(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.StateRollup{}); err != nil {
		t.Fatal(err)
	}
	node := bytes.Repeat([]byte{9}, 16)
	now := time.Date(2026, 8, 18, 12, 35, 0, 0, time.UTC)
	currentHour := now.Truncate(time.Hour)
	prevHour := currentHour.Add(-time.Hour)
	if err := db.Create([]model.StateRollup{
		{NodeUUID: node, Resolution: "1h", WindowStart: prevHour.UnixNano(), WindowEnd: currentHour.UnixNano(), Payload: []byte{1}, NetInTotal: 40, NetOutTotal: 10},
		{NodeUUID: node, Resolution: "1m", WindowStart: currentHour.UnixNano(), WindowEnd: currentHour.Add(time.Minute).UnixNano(), Payload: []byte{1}, NetInTotal: 5, NetOutTotal: 1},
	}).Error; err != nil {
		t.Fatal(err)
	}
	points, err := hourlySeries(db, node, model.TrafficDirectionInbound, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 24 {
		t.Fatalf("len=%d", len(points))
	}
	if points[22].Bytes != 40 {
		t.Fatalf("prev hour=%d", points[22].Bytes)
	}
	if points[23].Bytes != 5 {
		t.Fatalf("current hour=%d", points[23].Bytes)
	}
	total, err := hourlySeries(db, node, model.TrafficDirectionTotal, now)
	if err != nil {
		t.Fatal(err)
	}
	if total[22].Bytes != 50 || total[23].Bytes != 6 {
		t.Fatalf("total prev=%d current=%d", total[22].Bytes, total[23].Bytes)
	}
}

func TestDailySeriesGroupsByLocationDate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.StateRollup{}); err != nil {
		t.Fatal(err)
	}
	node := bytes.Repeat([]byte{8}, 16)
	shanghai := LocationOrUTC("Asia/Shanghai")
	now := time.Date(2026, 8, 18, 18, 20, 0, 0, time.UTC)        // 2026-08-19 02:20 CST
	windowStart := time.Date(2026, 8, 17, 16, 0, 0, 0, time.UTC) // 2026-08-18 00:00 CST
	hourOn18th := time.Date(2026, 8, 17, 16, 0, 0, 0, time.UTC)
	hourOn19th := time.Date(2026, 8, 18, 16, 0, 0, 0, time.UTC)
	if err := db.Create([]model.StateRollup{
		{NodeUUID: node, Resolution: "1h", WindowStart: hourOn18th.UnixNano(), WindowEnd: hourOn18th.Add(time.Hour).UnixNano(), Payload: []byte{1}, NetOutTotal: 100},
		{NodeUUID: node, Resolution: "1h", WindowStart: hourOn19th.UnixNano(), WindowEnd: hourOn19th.Add(time.Hour).UnixNano(), Payload: []byte{1}, NetOutTotal: 20},
		{NodeUUID: node, Resolution: "1m", WindowStart: now.Truncate(time.Hour).UnixNano(), WindowEnd: now.Truncate(time.Hour).Add(time.Minute).UnixNano(), Payload: []byte{1}, NetOutTotal: 3},
	}).Error; err != nil {
		t.Fatal(err)
	}
	points, err := dailySeries(db, node, model.TrafficDirectionOutbound, windowStart, now, shanghai)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 {
		t.Fatalf("days=%d", len(points))
	}
	if points[0].Bytes != 100 || points[1].Bytes != 23 {
		t.Fatalf("day0=%d day1=%d", points[0].Bytes, points[1].Bytes)
	}
}

func TestHistoriesWithoutBindingReturnsZeroSeries(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrafficPolicy{}, &model.ServerNodeBinding{}, &model.StateRollup{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	policy := model.TrafficPolicy{ServerID: 4, Name: "month", Direction: model.TrafficDirectionTotal, Mode: model.TrafficModeCumulative, QuotaBytes: 1000, WarningPercent: 80, Enabled: true}
	policy.CreatedAt = now.Add(-24 * time.Hour)
	if err := db.Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	items, err := Histories(db, 4, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Usage.UsedBytes != 0 || len(items[0].Hourly) != 24 {
		t.Fatalf("%#v", items)
	}
}

func TestSummariesOnlyEnabledPolicies(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrafficPolicy{}, &model.ServerNodeBinding{}, &model.StateRollup{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	on := model.TrafficPolicy{ServerID: 2, Name: "on", Direction: model.TrafficDirectionInbound, Mode: model.TrafficModeCumulative, QuotaBytes: 100, WarningPercent: 80, Enabled: true}
	off := model.TrafficPolicy{ServerID: 2, Name: "off", Direction: model.TrafficDirectionInbound, Mode: model.TrafficModeCumulative, QuotaBytes: 100, WarningPercent: 80, Enabled: false}
	on.CreatedAt = now.Add(-time.Hour)
	off.CreatedAt = now.Add(-time.Hour)
	if err := db.Create(&on).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&off).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&off).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	got, err := Summaries(db, []uint64{2}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got[2]) != 1 || got[2][0].Name != "on" {
		t.Fatalf("%#v", got)
	}
}

func TestHistoriesWithBindingFillsHourlyAndDaily(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrafficPolicy{}, &model.ServerNodeBinding{}, &model.StateRollup{}); err != nil {
		t.Fatal(err)
	}
	node := bytes.Repeat([]byte{7}, 16)
	now := time.Date(2026, 8, 18, 12, 10, 0, 0, time.UTC)
	policy := model.TrafficPolicy{ServerID: 5, Name: "month", Direction: model.TrafficDirectionInbound, Mode: model.TrafficModeCumulative, QuotaBytes: 1000, WarningPercent: 80, Enabled: true}
	policy.CreatedAt = now.Add(-48 * time.Hour)
	if err := db.Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ServerNodeBinding{ServerID: 5, NodeUUID: node, Current: true, Reason: "test", ValidFrom: now.Add(-48 * time.Hour).UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	prevHour := now.Truncate(time.Hour).Add(-time.Hour)
	if err := db.Create([]model.StateRollup{
		{NodeUUID: node, Resolution: "1h", WindowStart: prevHour.UnixNano(), WindowEnd: prevHour.Add(time.Hour).UnixNano(), Payload: []byte{1}, NetInTotal: 40},
		{NodeUUID: node, Resolution: "1m", WindowStart: now.Truncate(time.Hour).UnixNano(), WindowEnd: now.Truncate(time.Hour).Add(time.Minute).UnixNano(), Payload: []byte{1}, NetInTotal: 5},
	}).Error; err != nil {
		t.Fatal(err)
	}
	items, err := Histories(db, 5, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || len(items[0].Hourly) != 24 || items[0].Hourly[22].Bytes != 40 || items[0].Hourly[23].Bytes != 5 {
		t.Fatalf("%#v", items)
	}
	if items[0].Usage.UsedBytes == 0 {
		t.Fatal("expected current window usage")
	}
}
