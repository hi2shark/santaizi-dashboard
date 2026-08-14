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

func TestValidateAllRejectsInvalidPolicy(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	valid := model.TrafficPolicy{Name: "ok", Direction: model.TrafficDirectionTotal, Mode: model.TrafficModeRecurring, CycleStart: &start, CycleInterval: 1, CycleUnit: "month", QuotaBytes: 1, WarningPercent: 80}
	if err := ValidateAll([]model.TrafficPolicy{valid}); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.Name = ""
	if err := ValidateAll([]model.TrafficPolicy{valid, invalid}); err == nil {
		t.Fatal("expected invalid policy to fail")
	}
}

func TestReplaceUpsertsInPlaceAndDeletesMissingWithState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrafficPolicy{}, &model.TrafficPolicyState{}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	keep := model.TrafficPolicy{ServerID: 7, Name: "keep", Direction: model.TrafficDirectionTotal, Mode: model.TrafficModeRecurring, CycleStart: &start, CycleInterval: 1, CycleUnit: "month", QuotaBytes: 100, WarningPercent: 80, Enabled: true}
	drop := model.TrafficPolicy{ServerID: 7, Name: "drop", Direction: model.TrafficDirectionInbound, Mode: model.TrafficModeCumulative, QuotaBytes: 50, WarningPercent: 80, Enabled: true}
	if err := db.Create(&keep).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&drop).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TrafficPolicyState{PolicyID: keep.ID, UsedBytes: 12, LastEvaluatedAt: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TrafficPolicyState{PolicyID: drop.ID, UsedBytes: 9, LastEvaluatedAt: 1}).Error; err != nil {
		t.Fatal(err)
	}
	updated := keep
	updated.Name = "keep-renamed"
	updated.QuotaBytes = 200
	created := model.TrafficPolicy{Name: "new", Direction: model.TrafficDirectionOutbound, Mode: model.TrafficModeCumulative, QuotaBytes: 30, WarningPercent: 80, Enabled: true}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return Replace(tx, 7, []model.TrafficPolicy{updated, created})
	}); err != nil {
		t.Fatal(err)
	}
	var rows []model.TrafficPolicy
	if err := db.Where("server_id = ?", 7).Order("id ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].ID != keep.ID || rows[0].Name != "keep-renamed" || rows[0].QuotaBytes != 200 {
		t.Fatalf("kept=%#v", rows[0])
	}
	if rows[1].Name != "new" || rows[1].ID == 0 {
		t.Fatalf("created=%#v", rows[1])
	}
	var keepState model.TrafficPolicyState
	if err := db.First(&keepState, "policy_id = ?", keep.ID).Error; err != nil {
		t.Fatal(err)
	}
	if keepState.UsedBytes != 12 {
		t.Fatalf("state was reset: %#v", keepState)
	}
	var dropState model.TrafficPolicyState
	if err := db.First(&dropState, "policy_id = ?", drop.ID).Error; err == nil {
		t.Fatalf("dropped state still exists: %#v", dropState)
	}
	var dropRow model.TrafficPolicy
	if err := db.Unscoped().First(&dropRow, drop.ID).Error; err == nil && dropRow.ID != 0 && dropRow.DeletedAt.Valid == false {
		t.Fatalf("dropped policy still present: %#v", dropRow)
	}
}

func TestReplaceRejectsForeignPolicyID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrafficPolicy{}, &model.TrafficPolicyState{}); err != nil {
		t.Fatal(err)
	}
	other := model.TrafficPolicy{ServerID: 8, Name: "other", Direction: model.TrafficDirectionTotal, Mode: model.TrafficModeCumulative, QuotaBytes: 1, WarningPercent: 80}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	err = Replace(db, 7, []model.TrafficPolicy{{Common: model.Common{ID: other.ID}, Name: "stolen", Direction: model.TrafficDirectionTotal, Mode: model.TrafficModeCumulative, QuotaBytes: 1, WarningPercent: 80}})
	if err == nil {
		t.Fatal("expected foreign id to fail")
	}
}
