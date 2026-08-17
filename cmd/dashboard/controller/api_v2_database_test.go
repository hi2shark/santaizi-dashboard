package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hi2shark/santaizi-dashboard/service/telemetry"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestV2GetDatabaseReportsFileSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	path := filepath.Join(t.TempDir(), "santaizi.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.Exec("CREATE TABLE keep (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	original := DatabaseMaintainer
	t.Cleanup(func() { DatabaseMaintainer = original })
	DatabaseMaintainer = telemetry.NewDatabaseMaintainer(db, path, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v2/admin/database", nil)
	v2GetDatabase(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data telemetry.DatabaseStatus `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.FileBytes <= 0 {
		t.Fatalf("file_bytes=%d", payload.Data.FileBytes)
	}
	if payload.Data.Running {
		t.Fatal("idle GET should not be running")
	}
}

func TestV2OptimizeDatabaseConflictWhenRunning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	hold := make(chan struct{})
	release := make(chan struct{})
	original := DatabaseMaintainer
	t.Cleanup(func() {
		close(release)
		for i := 0; i < 50 && DatabaseMaintainer != nil && DatabaseMaintainer.Status().Running; i++ {
			time.Sleep(20 * time.Millisecond)
		}
		DatabaseMaintainer = original
	})
	DatabaseMaintainer = telemetry.NewDatabaseMaintainer(db, "", func() telemetry.RetentionPolicy {
		close(hold)
		<-release
		return telemetry.NormalizeRetentionPolicy(telemetry.RetentionPolicy{})
	})

	started := httptest.NewRecorder()
	startCtx, _ := gin.CreateTestContext(started)
	startCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v2/admin/database/optimize", nil)
	v2OptimizeDatabase(startCtx)
	if started.Code != http.StatusOK {
		t.Fatalf("start status=%d body=%s", started.Code, started.Body.String())
	}
	select {
	case <-hold:
	case <-time.After(2 * time.Second):
		t.Fatal("optimize did not start")
	}

	conflict := httptest.NewRecorder()
	conflictCtx, _ := gin.CreateTestContext(conflict)
	conflictCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v2/admin/database/optimize", nil)
	v2OptimizeDatabase(conflictCtx)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}
	var problem v2Problem
	if err := json.Unmarshal(conflict.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "optimize_in_progress" {
		t.Fatalf("code=%s", problem.Code)
	}
}
