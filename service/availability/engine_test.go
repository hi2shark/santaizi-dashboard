package availability

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newEngineTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.ObserverAssignment{}, &model.ObserverHealthBucket{}, &model.ObserverPathBucket{},
		&model.AvailabilityBucket{}, &model.AvailabilityIncident{}, &model.IncidentRevision{},
		&model.AvailabilityRecomputeQueue{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestClassifyPartialStillOnlineAndAvailable(t *testing.T) {
	tests := []struct {
		name                        string
		healthy, seen, minObservers uint32
		host, connectivity          string
	}{
		{name: "scenario A partial", healthy: 2, seen: 1, minObservers: 1, host: model.HostStateOnline, connectivity: model.ConnectivityPartial},
		{name: "scenario B full", healthy: 3, seen: 3, minObservers: 1, host: model.HostStateOnline, connectivity: model.ConnectivityFull},
		{name: "scenario C unavailable", healthy: 3, seen: 0, minObservers: 1, host: model.HostStateOffline, connectivity: model.ConnectivityUnavailable},
		{name: "scenario D observer loss", healthy: 0, seen: 0, minObservers: 1, host: model.HostStateUnknown, connectivity: model.ConnectivityUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			host, connectivity := classify(test.healthy, test.seen, test.minObservers)
			if host != test.host || connectivity != test.connectivity {
				t.Fatalf("host=%s connectivity=%s", host, connectivity)
			}
		})
	}
}

func TestLateEvidenceCreatesImmutableIncidentRevision(t *testing.T) {
	db := newEngineTestDB(t)
	engine := NewEngine(db, 30*time.Second, 1)
	node := bytes.Repeat([]byte{7}, 16)
	now := time.Now()
	bucket := now.Add(-time.Minute).UnixNano() / int64(30*time.Second) * int64(30*time.Second)
	for _, observer := range []string{"primary", "collector-a"} {
		if err := db.Create(&model.ObserverAssignment{NodeUUID: node, ObserverID: observer, ValidFrom: bucket - 1, Generation: 1, ConfigVersion: 1}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.ObserverHealthBucket{ObserverID: observer, BucketStart: bucket, Healthy: true, Revision: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.Recompute(context.Background(), node, bucket, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ObserverPathBucket{NodeUUID: node, ObserverID: "collector-a", BucketStart: bucket, Seen: true, Revision: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := engine.Recompute(context.Background(), node, bucket, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var availability model.AvailabilityBucket
	if err := db.First(&availability, "node_uuid = ? AND bucket_start = ?", node, bucket).Error; err != nil {
		t.Fatal(err)
	}
	if availability.HostState != model.HostStateOnline || availability.ConnectivityState != model.ConnectivityPartial || availability.Revision != 2 {
		t.Fatalf("availability=%#v", availability)
	}
	var incident model.AvailabilityIncident
	if err := db.First(&incident, "node_uuid = ? AND started_at = ?", node, bucket).Error; err != nil {
		t.Fatal(err)
	}
	if incident.InitialClassification != "HOST_OFFLINE" || incident.CurrentClassification != "CONNECTIVITY_DEGRADED" || incident.Revision != 2 {
		t.Fatalf("incident=%#v", incident)
	}
	var revisions int64
	if err := db.Model(&model.IncidentRevision{}).Where("incident_id = ?", incident.ID).Count(&revisions).Error; err != nil {
		t.Fatal(err)
	}
	if revisions != 2 {
		t.Fatalf("revision count=%d", revisions)
	}
}

func TestConsecutiveOfflineBucketsShareOneIncident(t *testing.T) {
	db := newEngineTestDB(t)
	engine := NewEngine(db, 30*time.Second, 1)
	node := bytes.Repeat([]byte{9}, 16)
	now := time.Now()
	bucketSize := int64(30 * time.Second)
	first := now.Add(-time.Minute).UnixNano() / bucketSize * bucketSize
	second := first + bucketSize
	seedObserverHealth(t, db, node, first, "primary")
	seedObserverHealth(t, db, node, second, "primary")
	if err := engine.Recompute(context.Background(), node, first, now); err != nil {
		t.Fatal(err)
	}
	if err := engine.Recompute(context.Background(), node, second, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var incidents []model.AvailabilityIncident
	if err := db.Where("node_uuid = ?", node).Order("id ASC").Find(&incidents).Error; err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 1 {
		t.Fatalf("incidents=%d", len(incidents))
	}
	incident := incidents[0]
	if incident.StartedAt != first || incident.EndedAt != 0 || incident.CurrentClassification != "HOST_OFFLINE" || incident.Revision != 1 {
		t.Fatalf("incident=%#v", incident)
	}
}

func TestHealthyBucketClosesIncidentAndLaterOfflineStartsNew(t *testing.T) {
	db := newEngineTestDB(t)
	engine := NewEngine(db, 30*time.Second, 1)
	node := bytes.Repeat([]byte{11}, 16)
	now := time.Now()
	bucketSize := int64(30 * time.Second)
	first := now.Add(-90*time.Second).UnixNano() / bucketSize * bucketSize
	healthy := first + bucketSize
	again := healthy + bucketSize
	seedObserverHealth(t, db, node, first, "primary")
	seedObserverHealth(t, db, node, healthy, "primary")
	seedObserverHealth(t, db, node, again, "primary")
	if err := engine.Recompute(context.Background(), node, first, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ObserverPathBucket{NodeUUID: node, ObserverID: "primary", BucketStart: healthy, Seen: true, Revision: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := engine.Recompute(context.Background(), node, healthy, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := engine.Recompute(context.Background(), node, again, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	var incidents []model.AvailabilityIncident
	if err := db.Where("node_uuid = ?", node).Order("started_at ASC").Find(&incidents).Error; err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 2 {
		t.Fatalf("incidents=%d %#v", len(incidents), incidents)
	}
	if incidents[0].StartedAt != first || incidents[0].EndedAt != healthy || incidents[0].CurrentClassification != "HOST_OFFLINE" {
		t.Fatalf("closed=%#v", incidents[0])
	}
	if incidents[1].StartedAt != again || incidents[1].EndedAt != 0 || incidents[1].CurrentClassification != "HOST_OFFLINE" || incidents[1].ID == incidents[0].ID {
		t.Fatalf("reopened=%#v", incidents[1])
	}
}

func TestLegacyPerBucketIncidentIsExtended(t *testing.T) {
	db := newEngineTestDB(t)
	engine := NewEngine(db, 30*time.Second, 1)
	node := bytes.Repeat([]byte{13}, 16)
	now := time.Now()
	bucketSize := int64(30 * time.Second)
	first := now.Add(-time.Minute).UnixNano() / bucketSize * bucketSize
	second := first + bucketSize
	seedObserverHealth(t, db, node, first, "primary")
	seedObserverHealth(t, db, node, second, "primary")
	if err := db.Create(&model.AvailabilityIncident{
		NodeUUID: node, InitialClassification: "HOST_OFFLINE", CurrentClassification: "HOST_OFFLINE",
		Revision: 1, StartedAt: first, EndedAt: second, Reason: "availability evidence",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := engine.Recompute(context.Background(), node, second, now); err != nil {
		t.Fatal(err)
	}
	var incidents []model.AvailabilityIncident
	if err := db.Where("node_uuid = ?", node).Find(&incidents).Error; err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 1 || incidents[0].EndedAt != 0 || incidents[0].StartedAt != first {
		t.Fatalf("incidents=%#v", incidents)
	}
}

func seedObserverHealth(t *testing.T, db *gorm.DB, node []byte, bucket int64, observers ...string) {
	t.Helper()
	for _, observer := range observers {
		var existing int64
		if err := db.Model(&model.ObserverAssignment{}).Where("node_uuid = ? AND observer_id = ?", node, observer).Count(&existing).Error; err != nil {
			t.Fatal(err)
		}
		if existing == 0 {
			if err := db.Create(&model.ObserverAssignment{NodeUUID: node, ObserverID: observer, ValidFrom: bucket - 1, Generation: 1, ConfigVersion: 1}).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := db.Create(&model.ObserverHealthBucket{ObserverID: observer, BucketStart: bucket, Healthy: true, Revision: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
}
