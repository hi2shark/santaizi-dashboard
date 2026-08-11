package controller

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/hi2shark/santaizi-dashboard/model"
	"github.com/hi2shark/santaizi-dashboard/resource"
)

func TestTelemetryTranslationsExist(t *testing.T) {
	messageIDs := []string{
		"Telemetry", "TelemetryReliability", "TelemetryReliabilityDescription", "TelemetryCollectors",
		"TelemetryObserverAssignments", "TelemetryAgentDelivery", "TelemetryIncidentRevisions",
		"TelemetryDataLoss", "TelemetryAlerts", "TelemetryCollectorID", "TelemetryAddress",
		"TelemetryGeneration", "TelemetryConfigVersion", "TelemetryObserver", "TelemetryNodeID",
		"TelemetryEffectiveFrom", "TelemetryLocalBufferPressure", "TelemetryBufferedData",
		"TelemetryPendingRecords", "TelemetryProtocolVersion", "TelemetryInitialClassification",
		"TelemetryCurrentClassification", "TelemetryRevision", "TelemetryStartedAt",
		"TelemetryComponent", "TelemetryLossReason", "TelemetryLostRecords", "TelemetryAlertType",
		"TelemetrySeverity", "TelemetryMessage", "TelemetryNotified", "TelemetryActive",
		"TelemetryRevoked", "TelemetryYes", "TelemetryNo", "TelemetryUnknown", "TelemetryWalHealthy",
		"TelemetryWalDownsampled", "TelemetryWalRollup", "TelemetryWalCritical", "TelemetryWalDataLoss",
		"TelemetryLossCompacted", "TelemetryLossHardLimit", "TelemetryLossCorruption",
		"TelemetryAlertHostOffline", "TelemetryAlertConnectivityDegraded", "TelemetryAlertCollectorOffline",
		"TelemetryAlertDataLoss", "TelemetrySeverityCritical", "TelemetrySeverityWarning", "TelemetrySeverityInfo",
	}
	for language := range model.Languages {
		content, err := resource.I18nFS.ReadFile("l10n/" + language + ".toml")
		if err != nil {
			t.Fatal(err)
		}
		messages := make(map[string]struct {
			Other string `toml:"other"`
		})
		if _, err := toml.Decode(string(content), &messages); err != nil {
			t.Fatalf("parse %s: %v", language, err)
		}
		for _, messageID := range messageIDs {
			if strings.TrimSpace(messages[messageID].Other) == "" {
				t.Errorf("%s is missing %s", language, messageID)
			}
		}
	}
}
