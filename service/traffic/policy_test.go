package traffic

import (
	"bytes"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCalculateUsesSafeRollupDeltasWithoutDoubleCounting(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ServerNodeBinding{}, &model.StateRollup{}); err != nil {
		t.Fatal(err)
	}
	node := bytes.Repeat([]byte{1}, 16)
	if err := db.Create(&model.ServerNodeBinding{ServerID: 7, NodeUUID: node, Current: true}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 11, 12, 35, 0, 0, time.UTC)
	rows := []model.StateRollup{
		{NodeUUID: node, Resolution: "1h", WindowStart: now.Add(-time.Hour).Truncate(time.Hour).UnixNano(), WindowEnd: now.Truncate(time.Hour).UnixNano(), Payload: []byte{1}, NetInTotal: 100, NetOutTotal: 200},
		{NodeUUID: node, Resolution: "1m", WindowStart: now.Truncate(time.Hour).UnixNano(), WindowEnd: now.Truncate(time.Hour).Add(time.Minute).UnixNano(), Payload: []byte{1}, NetInTotal: 10, NetOutTotal: 20},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	start := now.Add(-2 * time.Hour)
	usage, err := Calculate(db, model.TrafficPolicy{Common: model.Common{ID: 3, CreatedAt: start}, ServerID: 7, Name: "quota", Direction: model.TrafficDirectionTotal, Mode: model.TrafficModeCumulative, QuotaBytes: 1000, WarningPercent: 80}, now)
	if err != nil {
		t.Fatal(err)
	}
	if usage.UsedBytes != 330 {
		t.Fatalf("used=%d", usage.UsedBytes)
	}
}

func TestRecurringCalendarWindow(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	policy := model.TrafficPolicy{Name: "monthly", Direction: model.TrafficDirectionInbound, Mode: model.TrafficModeRecurring, CycleStart: &start, CycleInterval: 1, CycleUnit: "month", QuotaBytes: 1, WarningPercent: 80}
	from, to, err := Window(policy, time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if from.Month() != time.August || to == nil || to.Month() != time.September {
		t.Fatalf("window=%s..%v", from, to)
	}
}

func TestSumRollupUsageFillsPartialHourEdgesWithMinuteRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.StateRollup{}); err != nil {
		t.Fatal(err)
	}
	node := bytes.Repeat([]byte{2}, 16)
	start := time.Date(2026, 8, 11, 10, 35, 0, 0, time.UTC)
	end := time.Date(2026, 8, 11, 12, 20, 0, 0, time.UTC)
	rows := []model.StateRollup{
		{NodeUUID: node, Resolution: "1m", WindowStart: start.UnixNano(), WindowEnd: start.Add(time.Minute).UnixNano(), Payload: []byte{1}, NetInTotal: 5},
		{NodeUUID: node, Resolution: "1h", WindowStart: time.Date(2026, 8, 11, 11, 0, 0, 0, time.UTC).UnixNano(), WindowEnd: time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC).UnixNano(), Payload: []byte{1}, NetInTotal: 100},
		{NodeUUID: node, Resolution: "1m", WindowStart: time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC).UnixNano(), WindowEnd: time.Date(2026, 8, 11, 12, 1, 0, 0, time.UTC).UnixNano(), Payload: []byte{1}, NetInTotal: 7},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	used, err := sumRollupUsage(db, node, model.TrafficDirectionInbound, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if used != 112 {
		t.Fatalf("used=%d", used)
	}
}

func TestEvaluateAllNotifiesEachThresholdOncePerWindow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrafficPolicy{}, &model.TrafficPolicyState{}, &model.ServerNodeBinding{}, &model.StateRollup{}); err != nil {
		t.Fatal(err)
	}
	node := bytes.Repeat([]byte{3}, 16)
	now := time.Date(2026, 8, 11, 12, 30, 0, 0, time.UTC)
	policy := model.TrafficPolicy{ServerID: 9, Name: "monthly", Direction: model.TrafficDirectionInbound, Mode: model.TrafficModeCumulative, QuotaBytes: 100, WarningPercent: 80, NotificationTag: "ops", Enabled: true}
	policy.CreatedAt = now.Add(-time.Hour)
	if err := db.Create(&policy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ServerNodeBinding{ServerID: 9, NodeUUID: node, Current: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.StateRollup{NodeUUID: node, Resolution: "1m", WindowStart: now.Add(-20 * time.Minute).UnixNano(), WindowEnd: now.Add(-19 * time.Minute).UnixNano(), Payload: []byte{1}, NetInTotal: 85}).Error; err != nil {
		t.Fatal(err)
	}
	var warningCount, exceededCount int
	notify := func(_ string, message string, _ uint64) {
		if message == "monthly traffic quota exceeded" {
			exceededCount++
		} else {
			warningCount++
		}
	}
	if err := EvaluateAll(db, now, notify); err != nil {
		t.Fatal(err)
	}
	if err := EvaluateAll(db, now.Add(time.Minute), notify); err != nil {
		t.Fatal(err)
	}
	if warningCount != 1 || exceededCount != 0 {
		t.Fatalf("warning=%d exceeded=%d", warningCount, exceededCount)
	}
	if err := db.Create(&model.StateRollup{NodeUUID: node, Resolution: "1m", WindowStart: now.Add(-10 * time.Minute).UnixNano(), WindowEnd: now.Add(-9 * time.Minute).UnixNano(), Payload: []byte{1}, NetInTotal: 20}).Error; err != nil {
		t.Fatal(err)
	}
	if err := EvaluateAll(db, now.Add(2*time.Minute), notify); err != nil {
		t.Fatal(err)
	}
	if err := EvaluateAll(db, now.Add(3*time.Minute), notify); err != nil {
		t.Fatal(err)
	}
	if warningCount != 1 || exceededCount != 1 {
		t.Fatalf("warning=%d exceeded=%d", warningCount, exceededCount)
	}
}
