package telemetry

import (
	"context"
	"fmt"
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
	NotifyHostOffline      bool
	NotifyConnectivity     bool
	NotifyCorrection       bool
	NotifyCollectorOffline bool
	NotifyDataLoss         bool
	CollectorTimeout       time.Duration
}

type AlertWorker struct {
	db       *gorm.DB
	policy   AlertPolicy
	notifier func(string)
}

func NewAlertWorker(db *gorm.DB, policy AlertPolicy, notifier func(string)) *AlertWorker {
	if policy.CollectorTimeout <= 0 {
		policy.CollectorTimeout = CollectorTimeout
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
	var incidents []model.AvailabilityIncident
	if err := w.db.WithContext(ctx).Where("current_classification IN ?", []string{AlertHostOffline, AlertConnectivityDegraded}).Find(&incidents).Error; err != nil {
		return err
	}
	for _, incident := range incidents {
		typeName := incident.CurrentClassification
		notify := typeName == AlertHostOffline && w.policy.NotifyHostOffline || typeName == AlertConnectivityDegraded && w.policy.NotifyConnectivity
		if incident.Revision > 1 && incident.InitialClassification != incident.CurrentClassification && !w.policy.NotifyCorrection {
			notify = false
		}
		message := fmt.Sprintf("%s node=%x incident=%d revision=%d", typeName, incident.NodeUUID, incident.ID, incident.Revision)
		if err := w.create(ctx, model.TelemetryAlert{
			DedupKey: fmt.Sprintf("incident/%d/%d", incident.ID, incident.Revision), AlertType: typeName,
			Severity: "critical", NodeUUID: incident.NodeUUID, OccurredAt: incident.StartedAt, Message: message,
		}, notify); err != nil {
			return err
		}
	}
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
		message := fmt.Sprintf("%s collector=%s last_seen=%d", AlertCollectorOffline, collector.CollectorUUID, lastSeen)
		window := now.Unix() / int64(w.policy.CollectorTimeout.Seconds())
		if err := w.create(ctx, model.TelemetryAlert{
			DedupKey: fmt.Sprintf("collector/%s/%d", collector.CollectorUUID, window), AlertType: AlertCollectorOffline,
			Severity: "critical", ComponentID: collector.CollectorUUID, OccurredAt: now.UnixNano(), Message: message,
		}, w.policy.NotifyCollectorOffline); err != nil {
			return err
		}
	}
	var losses []model.TelemetryDataLoss
	if err := w.db.WithContext(ctx).Order("occurred_at DESC").Limit(1000).Find(&losses).Error; err != nil {
		return err
	}
	for _, loss := range losses {
		message := fmt.Sprintf("%s component=%s lost_records=%d", AlertDataLoss, loss.ComponentID, loss.LostRecords)
		if err := w.create(ctx, model.TelemetryAlert{
			DedupKey: fmt.Sprintf("data-loss/%x", loss.FactID), AlertType: AlertDataLoss, Severity: "critical",
			ComponentID: loss.ComponentID, OccurredAt: loss.OccurredAt, Message: message,
		}, w.policy.NotifyDataLoss); err != nil {
			return err
		}
	}
	var eventLosses []model.TelemetryEvent
	if err := w.db.WithContext(ctx).Where("event_type = ?", pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_DATA_LOSS).Order("collected_at DESC").Limit(1000).Find(&eventLosses).Error; err != nil {
		return err
	}
	for _, eventRow := range eventLosses {
		event := new(pb.TelemetryEvent)
		if proto.Unmarshal(eventRow.Payload, event) != nil || event.GetDataLoss() == nil {
			continue
		}
		message := fmt.Sprintf("%s component=agent_wal node=%x lost_records=%d", AlertDataLoss, eventRow.NodeUUID, event.GetDataLoss().GetLostRecords())
		if err := w.create(ctx, model.TelemetryAlert{
			DedupKey: fmt.Sprintf("event-data-loss/%x", eventRow.EventID), AlertType: AlertDataLoss,
			Severity: "critical", NodeUUID: eventRow.NodeUUID, ComponentID: "agent_wal",
			OccurredAt: eventRow.CollectedAt, Message: message,
		}, w.policy.NotifyDataLoss); err != nil {
			return err
		}
	}
	return nil
}

func (w *AlertWorker) create(ctx context.Context, alert model.TelemetryAlert, notify bool) error {
	alert.Notified = notify
	result := w.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&alert)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		fmt.Printf("SANTAIZI>> telemetry_alert type=%s severity=%s component=%s message=%q\n", alert.AlertType, alert.Severity, alert.ComponentID, alert.Message)
		if notify && w.notifier != nil {
			w.notifier(alert.Message)
		}
	}
	return nil
}
