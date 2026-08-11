package telemetry

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAlertWorkerPersistsDefaultMutedConnectivityAndDataLoss(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AvailabilityIncident{}, &model.Collector{}, &model.CollectorRuntime{}, &model.TelemetryDataLoss{}, &model.TelemetryEvent{}, &model.TelemetryAlert{}); err != nil {
		t.Fatal(err)
	}
	node := bytes.Repeat([]byte{1}, 16)
	if err := db.Create(&model.AvailabilityIncident{
		NodeUUID: node, InitialClassification: AlertConnectivityDegraded, CurrentClassification: AlertConnectivityDegraded,
		Revision: 1, StartedAt: time.Now().UnixNano(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TelemetryDataLoss{
		FactID: bytes.Repeat([]byte{2}, 16), ComponentID: "collector-a", OccurredAt: time.Now().UnixNano(), LostRecords: 3,
	}).Error; err != nil {
		t.Fatal(err)
	}
	notifications := 0
	worker := NewAlertWorker(db, AlertPolicy{}, func(string) { notifications++ })
	if err := worker.Scan(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	var alerts []model.TelemetryAlert
	if err := db.Order("alert_type ASC").Find(&alerts).Error; err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 2 || notifications != 0 {
		t.Fatalf("alerts=%d notifications=%d", len(alerts), notifications)
	}
	for _, alert := range alerts {
		if alert.Notified {
			t.Fatalf("default-muted alert was marked notified: %#v", alert)
		}
	}
}
