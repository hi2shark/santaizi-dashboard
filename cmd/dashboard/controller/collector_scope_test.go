package controller

import (
	"bytes"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCollectorScopeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Server{}, &model.ServerNodeBinding{}, &model.Collector{}, &model.CollectorScope{}, &model.ObserverAssignment{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestReplaceCollectorScopesSkipsProbeAssignments(t *testing.T) {
	db := newCollectorScopeDB(t)
	now := time.Unix(1_700_000_000, 0)
	node := bytes.Repeat([]byte{9}, 16)
	server := model.Server{Name: "edge-a", Secret: "secret"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ServerNodeBinding{ServerID: server.ID, NodeUUID: node, Current: true, Reason: "test", ValidFrom: now.UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	probe := model.Collector{
		CollectorUUID: "probe-x", Name: "probe", Kind: model.CollectorKindProbe,
		TokenHash: bytes.Repeat([]byte{3}, 32), RegistrationToken: "token-probe", Generation: 1, ConfigVersion: 1,
	}
	if err := db.Create(&probe).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ObserverAssignment{
		NodeUUID: node, ObserverID: probe.CollectorUUID, ValidFrom: now.Add(-time.Minute).UnixNano(), ConfigVersion: 1, Generation: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return replaceCollectorScopes(tx, &probe, []collectorScopeRequest{{Type: "all"}}, now)
	}); err != nil {
		t.Fatal(err)
	}

	var scopes int64
	if err := db.Model(&model.CollectorScope{}).Where("collector_uuid = ?", probe.CollectorUUID).Count(&scopes).Error; err != nil {
		t.Fatal(err)
	}
	if scopes != 1 {
		t.Fatalf("probe scopes=%d", scopes)
	}
	var active int64
	if err := db.Model(&model.ObserverAssignment{}).Where("observer_id = ? AND valid_to = 0", probe.CollectorUUID).Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("probe must not keep observer assignments, active=%d", active)
	}
}

func TestReplaceCollectorScopesCreatesObserverAssignment(t *testing.T) {
	db := newCollectorScopeDB(t)
	now := time.Unix(1_700_000_000, 0)
	node := bytes.Repeat([]byte{8}, 16)
	server := model.Server{Name: "edge-b", Secret: "secret"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ServerNodeBinding{ServerID: server.ID, NodeUUID: node, Current: true, Reason: "test", ValidFrom: now.UnixNano()}).Error; err != nil {
		t.Fatal(err)
	}
	observer := model.Collector{
		CollectorUUID: "obs-x", Name: "observer", Kind: model.CollectorKindObserver, Address: "obs.example:5556",
		TokenHash: bytes.Repeat([]byte{4}, 32), RegistrationToken: "token-obs", Generation: 1, ConfigVersion: 2,
	}
	if err := db.Create(&observer).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return replaceCollectorScopes(tx, &observer, []collectorScopeRequest{{Type: "all"}}, now)
	}); err != nil {
		t.Fatal(err)
	}

	var active int64
	if err := db.Model(&model.ObserverAssignment{}).Where("observer_id = ? AND valid_to = 0", observer.CollectorUUID).Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatalf("observer should receive assignment, active=%d", active)
	}
}
