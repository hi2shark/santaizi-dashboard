package singleton

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
)

const defaultNotificationAggregateWindow = 15 * time.Second

var (
	notificationAggregateWindow = defaultNotificationAggregateWindow
	notificationDeliver         = deliverNotification
	aggMu                       sync.Mutex
	aggBuckets                  = map[string]*notificationAggBucket{}
)

type notificationAggBucket struct {
	tag   string
	kind  string
	names []string
	seen  map[string]bool
	first string
	timer *time.Timer
}

func enqueueNotificationAggregate(tag, kind string, names []string, desc string) {
	if notificationAggregateWindow <= 0 {
		notificationDeliver(tag, desc, nil)
		return
	}
	key := tag + "\x00" + kind
	aggMu.Lock()
	defer aggMu.Unlock()
	bucket := aggBuckets[key]
	if bucket == nil {
		bucket = &notificationAggBucket{tag: tag, kind: kind, seen: map[string]bool{}, first: desc}
		aggBuckets[key] = bucket
		bucket.timer = time.AfterFunc(notificationAggregateWindow, func() { flushNotificationAggregate(key) })
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || bucket.seen[name] {
			continue
		}
		bucket.seen[name] = true
		bucket.names = append(bucket.names, name)
	}
}

func flushNotificationAggregate(key string) {
	aggMu.Lock()
	bucket := aggBuckets[key]
	delete(aggBuckets, key)
	if bucket != nil && bucket.timer != nil {
		bucket.timer.Stop()
	}
	aggMu.Unlock()
	if bucket == nil {
		return
	}
	desc := bucket.first
	if len(bucket.names) > 1 {
		desc = formatAggregatedNotification(bucket.kind, bucket.names)
	}
	notificationDeliver(bucket.tag, desc, nil)
}

func flushAllNotificationAggregates() {
	aggMu.Lock()
	keys := make([]string, 0, len(aggBuckets))
	for key := range aggBuckets {
		keys = append(keys, key)
	}
	aggMu.Unlock()
	for _, key := range keys {
		flushNotificationAggregate(key)
	}
}

func resetNotificationAggregates() {
	aggMu.Lock()
	defer aggMu.Unlock()
	for _, bucket := range aggBuckets {
		if bucket.timer != nil {
			bucket.timer.Stop()
		}
	}
	aggBuckets = map[string]*notificationAggBucket{}
}

func classifyNotification(desc string) (kind string, names []string, ok bool) {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return "", nil, false
	}
	if strings.HasPrefix(desc, "HOST_OFFLINE") || strings.Contains(desc, "\nHOST_OFFLINE") {
		return "离线", extractLegacyField(desc, "node="), true
	}
	if strings.HasPrefix(desc, "COLLECTOR_OFFLINE") || strings.Contains(desc, "\nCOLLECTOR_OFFLINE") {
		return "从端离线", extractLegacyField(desc, "collector="), true
	}
	if strings.HasPrefix(desc, "CONNECTIVITY_DEGRADED") {
		return "连通降级", extractLegacyField(desc, "node="), true
	}
	if strings.HasPrefix(desc, "TELEMETRY_DATA_LOSS") {
		return "探测丢失", extractLegacyField(desc, "component="), true
	}
	if !strings.HasPrefix(desc, "[") {
		return "", nil, false
	}
	end := strings.Index(desc, "]")
	if end <= 1 {
		return "", nil, false
	}
	kind = strings.TrimSpace(desc[1:end])
	switch kind {
	case "离线", "从端离线", "恢复", "连通降级", "探测丢失", "ProbeDown", "ProbeUp", "ProbeLatency":
		return kind, parseAggregatedNames(desc[end+1:]), true
	default:
		return "", nil, false
	}
}

func parseAggregatedNames(body string) []string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) == 0 {
		return nil
	}
	first := strings.TrimSpace(lines[0])
	if isCountHeader(first) {
		names := make([]string, 0, len(lines)-1)
		for _, line := range lines[1:] {
			if name := cleanAggregatedName(line); name != "" {
				names = append(names, name)
			}
		}
		return names
	}
	if name := cleanAggregatedName(first); name != "" {
		return []string{name}
	}
	return nil
}

func cleanAggregatedName(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	for _, sep := range []string{"  最后上报", " 最后上报"} {
		if i := strings.Index(line, sep); i > 0 {
			return strings.TrimSpace(line[:i])
		}
	}
	return line
}

func isCountHeader(line string) bool {
	line = strings.TrimSpace(line)
	i := strings.IndexFunc(line, func(r rune) bool { return !unicode.IsDigit(r) })
	if i <= 0 {
		return false
	}
	rest := strings.TrimSpace(line[i:])
	return rest == "台" || rest == "条" || rest == "台主机"
}

func extractLegacyField(desc, prefix string) []string {
	var names []string
	seen := map[string]bool{}
	for _, line := range strings.Split(desc, "\n") {
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		value := strings.Fields(line[idx+len(prefix):])
		if len(value) == 0 {
			continue
		}
		name := value[0]
		if prefix == "node=" && len(name) > 8 {
			name = name[:8]
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func formatAggregatedNotification(kind string, names []string) string {
	unit := "条"
	switch kind {
	case "离线", "从端离线", "恢复", "连通降级":
		unit = "台"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %d %s", kind, len(names), unit)
	for _, name := range names {
		b.WriteByte('\n')
		b.WriteString(name)
	}
	return b.String()
}
