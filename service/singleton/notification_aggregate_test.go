package singleton

import (
	"strings"
	"testing"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
)

func TestClassifyNotification(t *testing.T) {
	tests := []struct {
		desc  string
		kind  string
		names []string
		ok    bool
	}{
		{desc: "[离线] 东京-1\n最后上报：01/02/2026 15:04:05", kind: "离线", names: []string{"东京-1"}, ok: true},
		{desc: "[离线] 3 台主机\n东京-1  最后上报 01/02/2026 15:04:05\n大阪  最后上报 01/02/2026 15:04:06", kind: "离线", names: []string{"东京-1", "大阪"}, ok: true},
		{desc: "[从端离线] 大阪", kind: "从端离线", names: []string{"大阪"}, ok: true},
		{desc: "[恢复] 东京-1\n恢复时间：01/02/2026 15:04:05", kind: "恢复", names: []string{"东京-1"}, ok: true},
		{desc: "[连通降级] 东京-1", kind: "连通降级", names: []string{"东京-1"}, ok: true},
		{desc: "[探测丢失] collector-a", kind: "探测丢失", names: []string{"collector-a"}, ok: true},
		{desc: "[ProbeDown] 大阪 → 东京-1 timeout", kind: "ProbeDown", names: []string{"大阪 → 东京-1 timeout"}, ok: true},
		{desc: "HOST_OFFLINE node=abababababababab incident=3 revision=1", kind: "离线", names: []string{"abababab"}, ok: true},
		{desc: "[SSL] example.com expired", ok: false},
		{desc: "[IPChanged] 东京-1, 1.1.1.x => 2.2.2.x", ok: false},
	}
	for _, tt := range tests {
		kind, names, ok := classifyNotification(tt.desc)
		if ok != tt.ok || kind != tt.kind || !sameStrings(names, tt.names) {
			t.Fatalf("desc=%q kind=%q names=%q ok=%v", tt.desc, kind, names, ok)
		}
	}
}

func TestSendNotificationAggregatesSimilarMessages(t *testing.T) {
	got := captureNotificationDeliver(t)
	SendNotification("default", "[离线] 东京-1\n最后上报：01/02/2026 15:04:05", nil)
	SendNotification("default", "[离线] 大阪", nil)
	SendNotification("default", "[离线] 东京-1\n最后上报：01/02/2026 15:04:06", nil)
	if len(*got) != 0 {
		t.Fatalf("delivered before flush: %#v", *got)
	}
	flushAllNotificationAggregates()
	if len(*got) != 1 || (*got)[0] != "[离线] 2 台\n东京-1\n大阪" {
		t.Fatalf("got=%#v", *got)
	}
}

func TestSendNotificationKeepsSingleMessage(t *testing.T) {
	got := captureNotificationDeliver(t)
	msg := "[离线] 东京-1\n最后上报：01/02/2026 15:04:05\n判定离线：01/02/2026 15:04:35"
	SendNotification("default", msg, nil)
	flushAllNotificationAggregates()
	if len(*got) != 1 || (*got)[0] != msg {
		t.Fatalf("got=%#v", *got)
	}
}

func TestSendNotificationDoesNotMergeDifferentKinds(t *testing.T) {
	got := captureNotificationDeliver(t)
	SendNotification("default", "[离线] 东京-1", nil)
	SendNotification("default", "[恢复] 大阪", nil)
	flushAllNotificationAggregates()
	if len(*got) != 2 {
		t.Fatalf("got=%#v", *got)
	}
	if (*got)[0] != "[离线] 东京-1" || (*got)[1] != "[恢复] 大阪" {
		t.Fatalf("got=%#v", *got)
	}
}

func TestSendNotificationDeliversUnclassifiedImmediately(t *testing.T) {
	got := captureNotificationDeliver(t)
	SendNotification("default", "[SSL] example.com expired", nil)
	if len(*got) != 1 || (*got)[0] != "[SSL] example.com expired" {
		t.Fatalf("got=%#v", *got)
	}
}

func TestSendNotificationMergesExistingBatch(t *testing.T) {
	got := captureNotificationDeliver(t)
	SendNotification("default", "[离线] 3 台\n东京-1\n大阪\n首尔", nil)
	SendNotification("default", "[离线] 新加坡", nil)
	flushAllNotificationAggregates()
	if len(*got) != 1 || !strings.HasPrefix((*got)[0], "[离线] 4 台") || !strings.Contains((*got)[0], "新加坡") {
		t.Fatalf("got=%#v", *got)
	}
}

func captureNotificationDeliver(t *testing.T) *[]string {
	t.Helper()
	resetNotificationAggregates()
	prevWindow, prevDeliver := notificationAggregateWindow, notificationDeliver
	notificationAggregateWindow = time.Hour
	var got []string
	notificationDeliver = func(_, desc string, _ *model.Server) {
		got = append(got, desc)
	}
	t.Cleanup(func() {
		resetNotificationAggregates()
		notificationAggregateWindow = prevWindow
		notificationDeliver = prevDeliver
	})
	return &got
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
