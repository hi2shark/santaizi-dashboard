package singleton

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOpenDBFromPathCreatesVersionedSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "santaizi.db")
	db, err := OpenDBFromPath(path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := CloseDB(db); err != nil {
			t.Errorf("close db: %v", err)
		}
	})
	var migration model.SchemaMigration
	if err := db.First(&migration, 1).Error; err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&model.TelemetryEvent{}) || !db.Migrator().HasTable(&model.AvailabilityBucket{}) {
		t.Fatal("telemetry schema was not created")
	}
	if !db.Migrator().HasColumn(&model.Collector{}, "listen_port") {
		t.Fatal("collector listen_port column was not created")
	}
	if !db.Migrator().HasColumn(&model.Collector{}, "kind") {
		t.Fatal("collector kind column was not created")
	}
	if !db.Migrator().HasColumn(&model.Server{}, "probe_target") {
		t.Fatal("server probe_target column was not created")
	}
	if !db.Migrator().HasTable(&model.ProbeSampleBucket{}) || !db.Migrator().HasTable(&model.ProbeTrace{}) {
		t.Fatal("probe tables were not created")
	}
	if !db.Migrator().HasColumn(&model.CollectorRuntime{}, "software_version") {
		t.Fatal("collector_runtimes software_version column was not created")
	}
	if !db.Migrator().HasColumn(&model.Server{}, "probe_tcp_ports") || !db.Migrator().HasColumn(&model.Server{}, "probe_enable_icmp") {
		t.Fatal("server probe override columns were not created")
	}
	if !db.Migrator().HasColumn(&model.Collector{}, "enable_ipv4") || !db.Migrator().HasColumn(&model.Collector{}, "enable_ipv6") {
		t.Fatal("collector ip family columns were not created")
	}
}

func TestOpenDBFromPathRejectsUnversionedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE old_data (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := CloseDB(db); err != nil {
		t.Fatal(err)
	}

	if _, err := OpenDBFromPath(path, false); err == nil {
		t.Fatal("expected an unversioned database to be rejected")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database should remain available for diagnosis: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("rejected open left the sqlite file locked: %v", err)
	}
}

func TestMigrateV12AddsCollectorRuntimeSoftwareVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v11.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := CloseDB(db); err != nil {
			t.Errorf("close db: %v", err)
		}
	})
	if err := db.AutoMigrate(&model.SchemaMigration{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE collector_runtimes (
		collector_uuid TEXT PRIMARY KEY,
		status TEXT NOT NULL,
		last_seen INTEGER,
		protocol_version TEXT,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SchemaMigration{Version: 11, AppliedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasColumn(&model.CollectorRuntime{}, "software_version") {
		t.Fatal("fixture should omit software_version")
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&model.CollectorRuntime{}, "software_version") {
		t.Fatal("v12 should add software_version")
	}
	var current uint64
	if err := db.Model(&model.SchemaMigration{}).Select("COALESCE(MAX(version), 0)").Scan(&current).Error; err != nil {
		t.Fatal(err)
	}
	if current != 13 {
		t.Fatalf("version = %d", current)
	}
	if err := db.Create(&model.CollectorRuntime{CollectorUUID: "c1", Status: "online", SoftwareVersion: "1.2.3"}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestMigrateV13AddsServerProbeOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v12.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := CloseDB(db); err != nil {
			t.Errorf("close db: %v", err)
		}
	})
	if err := db.AutoMigrate(&model.SchemaMigration{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE servers (
		id INTEGER PRIMARY KEY,
		name TEXT,
		secret_ciphertext BLOB NOT NULL,
		probe_target TEXT
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE collectors (
		collector_uuid TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		address TEXT NOT NULL,
		token_ciphertext BLOB NOT NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SchemaMigration{Version: 12, AppliedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasColumn(&model.Server{}, "probe_tcp_ports") {
		t.Fatal("fixture should omit probe_tcp_ports")
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&model.Server{}, "probe_tcp_ports") || !db.Migrator().HasColumn(&model.Collector{}, "enable_ipv4") {
		t.Fatal("v13 should add probe override columns")
	}
	var current uint64
	if err := db.Model(&model.SchemaMigration{}).Select("COALESCE(MAX(version), 0)").Scan(&current).Error; err != nil {
		t.Fatal(err)
	}
	if current != 13 {
		t.Fatalf("version = %d", current)
	}
}
