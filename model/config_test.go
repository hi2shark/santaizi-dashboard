package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReliableTelemetryConfigUsesStableSnakeCaseKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard.yaml")
	contents := []byte(`mode: collector
telemetry:
  data_dir: /tmp/dashboard-telemetry
  state_interval_seconds: 7
  min_observers: 2
collector:
  primary_endpoint: primary.example:5555
  spool_max_bytes: 12345
retention:
  state_raw_hours: 12
  observation_days: 20
`)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	var config Config
	if err := config.Read(path); err != nil {
		t.Fatal(err)
	}
	if config.Telemetry.DataDir != "/tmp/dashboard-telemetry" || config.Telemetry.StateIntervalSeconds != 7 || config.Telemetry.MinObservers != 2 {
		t.Fatalf("telemetry=%#v", config.Telemetry)
	}
	if config.Collector.PrimaryEndpoint != "primary.example:5555" || config.Collector.SpoolMaxBytes != 12345 {
		t.Fatalf("collector=%#v", config.Collector)
	}
	if config.Retention.StateRawHours != 12 || config.Retention.ObservationDays != 20 {
		t.Fatalf("retention=%#v", config.Retention)
	}
}

func TestReliableTelemetryEnvironmentKeyMapping(t *testing.T) {
	tests := map[string]string{
		"SANTAIZI_HTTPPORT":                         "httpport",
		"SANTAIZI_SITE_BRAND":                       "site.brand",
		"SANTAIZI_TELEMETRY_STATE_INTERVAL_SECONDS": "telemetry.state_interval_seconds",
		"SANTAIZI_COLLECTOR_SPOOL_MAX_BYTES":        "collector.spool_max_bytes",
	}
	for input, expected := range tests {
		if actual := configEnvKey(input); actual != expected {
			t.Fatalf("%s -> %s, want %s", input, actual, expected)
		}
	}
}
