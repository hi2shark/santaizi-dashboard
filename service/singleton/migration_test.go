package singleton

import (
	"os"
	"path/filepath"
	"testing"

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
