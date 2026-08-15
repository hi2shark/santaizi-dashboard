package telemetry

import (
	"bytes"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func probeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Collector{}, &model.CollectorScope{}, &model.Server{}, &model.ServerRuntime{}, &model.ProbeSampleBucket{}, &model.ProbeLatest{}, &model.ProbeTrace{}, &model.ProbeAlertState{}, &model.CollectorRuntime{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestResolveProbeTargetOverrideAndNone(t *testing.T) {
	db := probeTestDB(t)
	override := ResolveProbeTarget(db, model.Server{Common: model.Common{ID: 1}, ProbeTarget: "origin.example"})
	if override.Source != "override" || override.Hostname != "origin.example" {
		t.Fatalf("%+v", override)
	}
	none := ResolveProbeTarget(db, model.Server{Common: model.Common{ID: 2}})
	if none.Source != "none" {
		t.Fatalf("%+v", none)
	}
}

func TestIngestProbeSampleNoTargetDoesNotAlert(t *testing.T) {
	db := probeTestDB(t)
	previous := singleton.DB
	singleton.DB = db
	t.Cleanup(func() { singleton.DB = previous })
	collector := model.Collector{CollectorUUID: "probe-a", Name: "HK", Kind: model.CollectorKindProbe, TokenHash: bytes.Repeat([]byte{1}, 32), RegistrationToken: "token-a", ProbeNotify: true, FailThreshold: 1, NotificationTag: "default"}
	if err := db.Create(&collector).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Server{Common: model.Common{ID: 9}, Name: "n9", Secret: "secret-9"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := IngestProbeSamples(db, &collector, &pb.ProbeSampleBatch{Samples: []*pb.ProbeSample{{
		ServerId: 9, SampledAtUnixNano: time.Now().UnixNano(), LastError: "timeout",
		Icmp: &pb.ProbeICMPSample{Ok: false, Error: "timeout"},
	}}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	var state model.ProbeAlertState
	if err := db.First(&state, "collector_uuid = ? AND server_id = ?", "probe-a", 9).Error; err == nil && state.DownNotified {
		t.Fatal("no-target path should not notify")
	}
}

func TestDisplayRTTPrefersICMPThenTCP(t *testing.T) {
	db := probeTestDB(t)
	collector := model.Collector{CollectorUUID: "probe-b", Name: "JP", Kind: model.CollectorKindProbe, TokenHash: bytes.Repeat([]byte{2}, 32), RegistrationToken: "token-b"}
	if err := db.Create(&collector).Error; err != nil {
		t.Fatal(err)
	}
	if err := IngestProbeSamples(db, &collector, &pb.ProbeSampleBatch{Samples: []*pb.ProbeSample{{
		ServerId: 3, SampledAtUnixNano: time.Now().UnixNano(),
		Icmp: &pb.ProbeICMPSample{Ok: false, Error: "blocked"},
		Tcp:  []*pb.ProbeTCPSample{{Port: 443, Ok: true, RttMs: 42}},
	}}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	var latest model.ProbeLatest
	if err := db.First(&latest, "collector_uuid = ? AND server_id = ?", "probe-b", 3).Error; err != nil {
		t.Fatal(err)
	}
	if !latest.Reachable || latest.DisplayRttMs != 42 {
		t.Fatalf("%+v", latest)
	}
}

func TestLoadProbePathsFiltersAndEmptyTrace(t *testing.T) {
	db := probeTestDB(t)
	probe := model.Collector{CollectorUUID: "probe-c", Name: "SG", Kind: model.CollectorKindProbe, TokenHash: bytes.Repeat([]byte{3}, 32), RegistrationToken: "token-c"}
	other := model.Collector{CollectorUUID: "probe-d", Name: "US", Kind: model.CollectorKindProbe, TokenHash: bytes.Repeat([]byte{4}, 32), RegistrationToken: "token-d"}
	if err := db.Create(&probe).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CollectorScope{CollectorUUID: "probe-c", ScopeType: "all"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CollectorScope{CollectorUUID: "probe-d", ScopeType: "all"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Server{Common: model.Common{ID: 4}, Name: "alpha", Secret: "secret-4", ProbeTarget: "1.1.1.1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Server{Common: model.Common{ID: 5}, Name: "beta", Secret: "secret-5"}).Error; err != nil {
		t.Fatal(err)
	}
	paths, err := LoadProbePaths(db, ProbePathFilter{CollectorID: "probe-c", ServerID: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0].CollectorID != "probe-c" || paths[0].ServerID != 4 || paths[0].TargetSource != "override" {
		t.Fatalf("%+v", paths)
	}
	none, err := LoadProbePaths(db, ProbePathFilter{CollectorID: "probe-c", ServerID: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 1 || none[0].TargetSource != "none" {
		t.Fatalf("%+v", none)
	}
	trace, err := GetProbeTrace(db, "probe-c", 4)
	if err != nil {
		t.Fatal(err)
	}
	if trace != nil {
		t.Fatalf("expected empty trace, got %+v", trace)
	}
}
