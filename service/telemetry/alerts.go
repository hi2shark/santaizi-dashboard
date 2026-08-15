package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AlertHostOffline          = "HOST_OFFLINE"
	AlertConnectivityDegraded = "CONNECTIVITY_DEGRADED"
	AlertCollectorOffline     = "COLLECTOR_OFFLINE"
	AlertDataLoss             = "TELEMETRY_DATA_LOSS"
)

type AlertPolicy struct {
	NotifyHostOffline         bool
	NotifyConnectivity        bool
	NotifyCorrection          bool
	NotifyCollectorOffline    bool
	NotifyDataLoss            bool
	SuppressHostOfflineNotify bool
	CollectorTimeout          time.Duration
	AvailabilityBucket        time.Duration
}

type AlertNotifier func(message, muteKey string)

type AlertWorker struct {
	db       *gorm.DB
	policy   AlertPolicy
	notifier AlertNotifier
}

type pendingNotice struct {
	alertType string
	title     string
	name      string
	muteKey   string
}

func NewAlertWorker(db *gorm.DB, policy AlertPolicy, notifier AlertNotifier) *AlertWorker {
	if policy.CollectorTimeout <= 0 {
		policy.CollectorTimeout = CollectorTimeout
	}
	if policy.AvailabilityBucket <= 0 {
		policy.AvailabilityBucket = 30 * time.Second
	}
	return &AlertWorker{db: db, policy: policy, notifier: notifier}
}

func (w *AlertWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			_ = w.Scan(ctx, now)
		}
	}
}

func (w *AlertWorker) Scan(ctx context.Context, now time.Time) error {
	var pending []pendingNotice
	if err := w.scanIncidents(ctx, now, &pending); err != nil {
		return err
	}
	if err := w.scanCollectors(ctx, now, &pending); err != nil {
		return err
	}
	if err := w.scanDataLoss(ctx, &pending); err != nil {
		return err
	}
	if err := w.scanEventDataLoss(ctx, &pending); err != nil {
		return err
	}
	w.flush(pending)
	return nil
}

func (w *AlertWorker) scanIncidents(ctx context.Context, now time.Time, pending *[]pendingNotice) error {
	cutoff := now.Add(-time.Hour).UnixNano()
	var incidents []model.AvailabilityIncident
	if err := w.db.WithContext(ctx).Where("current_classification IN ? AND (ended_at = 0 OR started_at >= ?)", []string{AlertHostOffline, AlertConnectivityDegraded}, cutoff).Find(&incidents).Error; err != nil {
		return err
	}
	episodes := assignEpisodeStarts(incidents, int64(w.policy.AvailabilityBucket))
	nodeIDs := make([][]byte, 0, len(incidents))
	for _, incident := range incidents {
		nodeIDs = append(nodeIDs, incident.NodeUUID)
	}
	idx, err := loadHostIndex(w.db.WithContext(ctx), uniqueByteSlices(nodeIDs), nil)
	if err != nil {
		return err
	}
	for i, incident := range incidents {
		typeName := incident.CurrentClassification
		notify := false
		switch typeName {
		case AlertHostOffline:
			notify = w.policy.NotifyHostOffline && !w.policy.SuppressHostOfflineNotify
		case AlertConnectivityDegraded:
			notify = w.policy.NotifyConnectivity
		}
		if incident.Revision > 1 {
			if incident.InitialClassification == incident.CurrentClassification {
				notify = false
			} else if !w.policy.NotifyCorrection {
				notify = false
			}
		}
		_, serverName := idx.host(incident.NodeUUID)
		name := displayHostName(serverName, incident.NodeUUID)
		title := alertTitle(typeName)
		message := formatNamedAlert(title, name)
		created, err := w.create(ctx, model.TelemetryAlert{
			DedupKey: fmt.Sprintf("episode/%s/%x/%d", typeName, incident.NodeUUID, episodes[i]), AlertType: typeName,
			Severity: "critical", NodeUUID: incident.NodeUUID, OccurredAt: incident.StartedAt, Message: message,
		}, notify)
		if err != nil {
			return err
		}
		if created && notify {
			*pending = append(*pending, pendingNotice{alertType: typeName, title: title, name: name, muteKey: fmt.Sprintf("bf::tho-%s-%x", typeName, incident.NodeUUID)})
		}
	}
	return nil
}

func (w *AlertWorker) scanCollectors(ctx context.Context, now time.Time, pending *[]pendingNotice) error {
	var collectors []model.Collector
	if err := w.db.WithContext(ctx).Where("revoked = ? AND deleted = ?", false, false).Find(&collectors).Error; err != nil {
		return err
	}
	for _, collector := range collectors {
		var runtime model.CollectorRuntime
		if err := w.db.WithContext(ctx).Where("collector_uuid = ?", collector.CollectorUUID).Limit(1).Find(&runtime).Error; err != nil {
			return err
		}
		lastSeen := runtime.LastSeen
		if (lastSeen == 0 && collector.CreatedAt.After(now.Add(-w.policy.CollectorTimeout))) || lastSeen >= now.Add(-w.policy.CollectorTimeout).UnixNano() {
			continue
		}
		name := displayCollectorName(collector.Name, collector.CollectorUUID)
		title := alertTitle(AlertCollectorOffline)
		message := formatNamedAlert(title, name)
		created, err := w.create(ctx, model.TelemetryAlert{
			DedupKey: fmt.Sprintf("collector/%s/%d", collector.CollectorUUID, lastSeen), AlertType: AlertCollectorOffline,
			Severity: "critical", ComponentID: collector.CollectorUUID, OccurredAt: now.UnixNano(), Message: message,
		}, w.policy.NotifyCollectorOffline)
		if err != nil {
			return err
		}
		if created && w.policy.NotifyCollectorOffline {
			*pending = append(*pending, pendingNotice{alertType: AlertCollectorOffline, title: title, name: name, muteKey: fmt.Sprintf("bf::tco-%s", collector.CollectorUUID)})
		}
	}
	return nil
}

func (w *AlertWorker) scanDataLoss(ctx context.Context, pending *[]pendingNotice) error {
	var losses []model.TelemetryDataLoss
	if err := w.db.WithContext(ctx).Order("occurred_at DESC").Limit(1000).Find(&losses).Error; err != nil {
		return err
	}
	for _, loss := range losses {
		message := formatNamedAlert(alertTitle(AlertDataLoss), loss.ComponentID)
		created, err := w.create(ctx, model.TelemetryAlert{
			DedupKey: fmt.Sprintf("data-loss/%x", loss.FactID), AlertType: AlertDataLoss, Severity: "critical",
			ComponentID: loss.ComponentID, OccurredAt: loss.OccurredAt, Message: message,
		}, w.policy.NotifyDataLoss)
		if err != nil {
			return err
		}
		if created && w.policy.NotifyDataLoss {
			*pending = append(*pending, pendingNotice{alertType: AlertDataLoss, title: alertTitle(AlertDataLoss), name: loss.ComponentID, muteKey: fmt.Sprintf("bf::tdl-%s", loss.ComponentID)})
		}
	}
	return nil
}

func (w *AlertWorker) scanEventDataLoss(ctx context.Context, pending *[]pendingNotice) error {
	var eventLosses []model.TelemetryEvent
	if err := w.db.WithContext(ctx).Where("event_type = ?", pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_DATA_LOSS).Order("collected_at DESC").Limit(1000).Find(&eventLosses).Error; err != nil {
		return err
	}
	for _, eventRow := range eventLosses {
		event := new(pb.TelemetryEvent)
		if proto.Unmarshal(eventRow.Payload, event) != nil || event.GetDataLoss() == nil {
			continue
		}
		idx, err := loadHostIndex(w.db.WithContext(ctx), [][]byte{eventRow.NodeUUID}, nil)
		if err != nil {
			return err
		}
		_, serverName := idx.host(eventRow.NodeUUID)
		name := displayHostName(serverName, eventRow.NodeUUID)
		title := alertTitle(AlertDataLoss)
		message := formatNamedAlert(title, name)
		created, err := w.create(ctx, model.TelemetryAlert{
			DedupKey: fmt.Sprintf("event-data-loss/%x", eventRow.EventID), AlertType: AlertDataLoss,
			Severity: "critical", NodeUUID: eventRow.NodeUUID, ComponentID: "agent_wal",
			OccurredAt: eventRow.CollectedAt, Message: message,
		}, w.policy.NotifyDataLoss)
		if err != nil {
			return err
		}
		if created && w.policy.NotifyDataLoss {
			*pending = append(*pending, pendingNotice{alertType: AlertDataLoss, title: title, name: name, muteKey: fmt.Sprintf("bf::tdl-agent_wal-%x", eventRow.NodeUUID)})
		}
	}
	return nil
}

func (w *AlertWorker) create(ctx context.Context, alert model.TelemetryAlert, notify bool) (bool, error) {
	alert.Notified = notify
	result := w.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&alert)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		fmt.Printf("SANTAIZI>> telemetry_alert type=%s severity=%s component=%s message=%q\n", alert.AlertType, alert.Severity, alert.ComponentID, alert.Message)
		return true, nil
	}
	return false, nil
}

func (w *AlertWorker) flush(pending []pendingNotice) {
	if w.notifier == nil || len(pending) == 0 {
		return
	}
	byType := map[string][]pendingNotice{}
	order := make([]string, 0)
	for _, item := range pending {
		if _, ok := byType[item.alertType]; !ok {
			order = append(order, item.alertType)
		}
		byType[item.alertType] = append(byType[item.alertType], item)
	}
	for _, typ := range order {
		items := byType[typ]
		if len(items) == 1 {
			w.notifier(formatNamedAlert(items[0].title, items[0].name), items[0].muteKey)
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "[%s] %d 台", items[0].title, len(items))
		for _, item := range items {
			b.WriteByte('\n')
			b.WriteString(item.name)
		}
		w.notifier(b.String(), fmt.Sprintf("bf::tburst-%s", typ))
	}
}

func alertTitle(typ string) string {
	switch typ {
	case AlertHostOffline:
		return "离线"
	case AlertConnectivityDegraded:
		return "连通降级"
	case AlertCollectorOffline:
		return "从端离线"
	case AlertDataLoss:
		return "探测丢失"
	default:
		return typ
	}
}

func formatNamedAlert(title, name string) string {
	return fmt.Sprintf("[%s] %s", title, name)
}

func displayHostName(serverName string, nodeUUID []byte) string {
	if name := strings.TrimSpace(serverName); name != "" {
		return name
	}
	hex := HexUUID(nodeUUID)
	if len(hex) > 8 {
		return hex[:8]
	}
	if hex == "" {
		return "未知主机"
	}
	return hex
}

func displayCollectorName(name, id string) string {
	if n := strings.TrimSpace(name); n != "" {
		return n
	}
	if id == "" {
		return "未知从端"
	}
	return id
}

func assignEpisodeStarts(incidents []model.AvailabilityIncident, bucket int64) []int64 {
	starts := make([]int64, len(incidents))
	if len(incidents) == 0 {
		return starts
	}
	if bucket <= 0 {
		bucket = int64(30 * time.Second)
	}
	order := make([]int, len(incidents))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := incidents[order[i]], incidents[order[j]]
		if c := bytes.Compare(a.NodeUUID, b.NodeUUID); c != 0 {
			return c < 0
		}
		if a.CurrentClassification != b.CurrentClassification {
			return a.CurrentClassification < b.CurrentClassification
		}
		return a.StartedAt < b.StartedAt
	})
	var prev *model.AvailabilityIncident
	var prevStart int64
	for _, i := range order {
		inc := incidents[i]
		if prev != nil && sameEpisode(*prev, inc, bucket) {
			starts[i] = prevStart
		} else {
			starts[i] = inc.StartedAt
			prevStart = inc.StartedAt
		}
		copyInc := inc
		prev = &copyInc
	}
	return starts
}

func sameEpisode(prev, curr model.AvailabilityIncident, bucket int64) bool {
	if !bytes.Equal(prev.NodeUUID, curr.NodeUUID) || prev.CurrentClassification != curr.CurrentClassification {
		return false
	}
	if prev.EndedAt == 0 || prev.EndedAt == curr.StartedAt {
		return true
	}
	if curr.StartedAt >= prev.StartedAt && curr.StartedAt-prev.StartedAt <= bucket {
		return true
	}
	return prev.EndedAt > 0 && curr.StartedAt <= prev.EndedAt+bucket
}
