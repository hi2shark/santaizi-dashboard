package controller

import (
	"strings"
	"testing"

	"github.com/hi2shark/santaizi-dashboard/model"
)

func TestBuildInstallCommandMatchesInstallerArguments(t *testing.T) {
	options := monitoringOptionsDTO{CPU: true, Memory: true, Disk: true, Network: true, HostInfo: true, IPReport: true, HTTPProbe: true, ICMPProbe: true, TCPProbe: true}
	posix, err := buildInstallCommand("linux", "https://example.invalid/install.sh", "grpc.example.invalid", 5555, "secret", true, options)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(posix, "'grpc.example.invalid' 5555 'secret' --clean-install --confirm-clean-install") || strings.Contains(posix, "--server") || strings.Contains(posix, "--secret") {
		t.Fatalf("posix=%s", posix)
	}
	windows, err := buildInstallCommand("windows", "https://example.invalid/install.ps1", "grpc.example.invalid", 5555, "secret", true, options)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(windows, "-Server 'grpc.example.invalid' -Port 5555 -Key 'secret' -CleanInstall -ConfirmCleanInstall") || strings.Contains(windows, "-Secret") {
		t.Fatalf("windows=%s", windows)
	}
}

func TestApplyAlertRuleWriteAllowsOfflineWithoutThreshold(t *testing.T) {
	request := alertRuleWriteDTO{
		Name:            "Host offline",
		Enabled:         true,
		TriggerMode:     "always",
		NotificationTag: "ops",
		Conditions: []alertConditionDTO{{
			Type:            "offline",
			DurationSeconds: 30,
			Scope:           monitorScopeDTO{Mode: "all", ServerIDs: []uint64{}},
		}},
	}
	var row model.AlertRule
	if err := applyAlertRuleWrite(&row, request); err != nil {
		t.Fatal(err)
	}
	if len(row.Rules) != 1 || row.Rules[0].Type != "offline" || row.Rules[0].Min != 0 || row.Rules[0].Max != 0 {
		t.Fatalf("unexpected rules: %#v", row.Rules)
	}
}

func TestNormalizeCollectorScopesRejectsIncompleteOrAmbiguousScopes(t *testing.T) {
	valid, err := normalizeCollectorScopes([]collectorScopeRequest{{Type: " SERVER ", Value: " 7 "}, {Type: "tag", Value: " edge "}})
	if err != nil {
		t.Fatal(err)
	}
	if valid[0].Type != "server" || valid[0].Value != "7" || valid[1].Value != "edge" {
		t.Fatalf("unexpected normalized scopes: %#v", valid)
	}
	for _, scopes := range [][]collectorScopeRequest{
		nil,
		{{Type: "all"}, {Type: "server", Value: "7"}},
		{{Type: "server", Value: ""}},
		{{Type: "tag", Value: ""}},
		{{Type: "group", Value: "edge"}, {Type: "group", Value: "edge"}},
	} {
		if _, err := normalizeCollectorScopes(scopes); err == nil {
			t.Fatalf("expected rejection for %#v", scopes)
		}
	}
}
