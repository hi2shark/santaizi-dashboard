package telemetry

import (
	"encoding/json"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"gorm.io/gorm"
)

const datasetLimit = 1000

type ObserverEvidenceItem struct {
	ObserverID   string `json:"observer_id"`
	ObserverKind string `json:"observer_kind,omitempty"`
	ObserverName string `json:"observer_name,omitempty"`
	Healthy      bool   `json:"healthy"`
	Seen         bool   `json:"seen"`
}

type AgentSink struct {
	EndpointID    string `json:"endpoint_id"`
	ObserverKind  string `json:"observer_kind,omitempty"`
	ObserverName  string `json:"observer_name,omitempty"`
	Connected     bool   `json:"connected"`
	PendingEvents uint64 `json:"pending_events"`
	LastError     string `json:"last_error,omitempty"`
	AckThrough    uint64 `json:"ack_through"`
}

type ObserverAssignmentRecord struct {
	ServerID      uint64  `json:"server_id,omitempty"`
	ServerName    string  `json:"server_name,omitempty"`
	NodeUUID      string  `json:"node_uuid"`
	ObserverID    string  `json:"observer_id"`
	ObserverKind  string  `json:"observer_kind"`
	ObserverName  string  `json:"observer_name,omitempty"`
	ValidFrom     *string `json:"valid_from"`
	ValidTo       *string `json:"valid_to"`
	Generation    uint64  `json:"generation"`
	ConfigVersion uint64  `json:"config_version"`
}

type AgentReliabilityRecord struct {
	ServerID        uint64      `json:"server_id,omitempty"`
	ServerName      string      `json:"server_name,omitempty"`
	NodeUUID        string      `json:"node_uuid"`
	WalPressure     string      `json:"wal_pressure"`
	WalBytes        uint64      `json:"wal_bytes"`
	PendingEvents   uint64      `json:"pending_events"`
	OldestPending   *string     `json:"oldest_pending"`
	ClockUntrusted  bool        `json:"clock_untrusted"`
	ProtocolVersion string      `json:"protocol_version,omitempty"`
	UpdatedAt       *string     `json:"updated_at"`
	Sinks           []AgentSink `json:"sinks"`
}

type IncidentRecord struct {
	ID                    uint64                 `json:"id"`
	ServerID              uint64                 `json:"server_id,omitempty"`
	ServerName            string                 `json:"server_name,omitempty"`
	NodeUUID              string                 `json:"node_uuid"`
	InitialClassification string                 `json:"initial_classification,omitempty"`
	CurrentClassification string                 `json:"current_classification"`
	Revision              uint64                 `json:"revision"`
	StartedAt             *string                `json:"started_at"`
	EndedAt               *string                `json:"ended_at"`
	RecalculatedAt        *string                `json:"recalculated_at"`
	Reason                string                 `json:"reason,omitempty"`
	ObserverEvidence      []ObserverEvidenceItem `json:"observer_evidence"`
}

type IncidentRevisionRecord struct {
	ID               uint64                 `json:"id"`
	IncidentID       uint64                 `json:"incident_id"`
	Revision         uint64                 `json:"revision"`
	Classification   string                 `json:"classification"`
	Reason           string                 `json:"reason,omitempty"`
	RecalculatedAt   *string                `json:"recalculated_at"`
	CreatedAt        *string                `json:"created_at"`
	ObserverEvidence []ObserverEvidenceItem `json:"observer_evidence"`
}

type DataLossRecord struct {
	FactID       string  `json:"fact_id"`
	ComponentID  string  `json:"component_id,omitempty"`
	OccurredAt   *string `json:"occurred_at"`
	Reason       string  `json:"reason"`
	FirstSpoolID uint64  `json:"first_spool_id,omitempty"`
	LastSpoolID  uint64  `json:"last_spool_id,omitempty"`
	LostRecords  uint64  `json:"lost_records"`
	Detail       string  `json:"detail,omitempty"`
	CreatedAt    *string `json:"created_at"`
}

type AlertRecord struct {
	ID          uint64  `json:"id"`
	AlertType   string  `json:"alert_type"`
	Severity    string  `json:"severity,omitempty"`
	ServerID    uint64  `json:"server_id,omitempty"`
	ServerName  string  `json:"server_name,omitempty"`
	NodeUUID    string  `json:"node_uuid,omitempty"`
	ComponentID string  `json:"component_id,omitempty"`
	OccurredAt  *string `json:"occurred_at"`
	CreatedAt   *string `json:"created_at"`
	Notified    bool    `json:"notified"`
	Message     string  `json:"message,omitempty"`
	DedupKey    string  `json:"dedup_key,omitempty"`
}

func ListObserverAssignments(db *gorm.DB) ([]ObserverAssignmentRecord, error) {
	out := []ObserverAssignmentRecord{}
	var rows []model.ObserverAssignment
	if err := db.Where("valid_to = ?", 0).Order("valid_from DESC").Limit(datasetLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	nodeIDs := uniqueBytes(rows, func(row model.ObserverAssignment) []byte { return row.NodeUUID })
	observerIDs := uniqueStrings(rows, func(row model.ObserverAssignment) string { return row.ObserverID })
	idx, err := loadHostIndex(db, nodeIDs, observerIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		serverID, serverName := idx.host(row.NodeUUID)
		kind, name := idx.observer(row.ObserverID)
		out = append(out, ObserverAssignmentRecord{
			ServerID: serverID, ServerName: serverName, NodeUUID: HexUUID(row.NodeUUID),
			ObserverID: row.ObserverID, ObserverKind: kind, ObserverName: name,
			ValidFrom: RFC3339NanoPtr(row.ValidFrom), ValidTo: RFC3339NanoPtr(row.ValidTo),
			Generation: row.Generation, ConfigVersion: row.ConfigVersion,
		})
	}
	return out, nil
}

func ListAgentReliability(db *gorm.DB) ([]AgentReliabilityRecord, error) {
	out := []AgentReliabilityRecord{}
	var rows []model.AgentTelemetryRuntime
	if err := db.Order("updated_at DESC").Limit(datasetLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	nodeIDs := make([][]byte, 0, len(rows))
	observerIDs := make([]string, 0)
	decoded := make([]*pb.AgentRuntime, len(rows))
	for i, row := range rows {
		nodeIDs = append(nodeIDs, row.NodeUUID)
		decoded[i] = DecodeAgentRuntime(row.SinkCursors)
		if decoded[i] == nil {
			continue
		}
		for _, sink := range decoded[i].GetSinks() {
			observerIDs = append(observerIDs, sink.GetEndpointId())
		}
	}
	idx, err := loadHostIndex(db, uniqueByteSlices(nodeIDs), observerIDs)
	if err != nil {
		return nil, err
	}
	for i, row := range rows {
		serverID, serverName := idx.host(row.NodeUUID)
		record := AgentReliabilityRecord{
			ServerID: serverID, ServerName: serverName, NodeUUID: HexUUID(row.NodeUUID),
			WalPressure: walPressureLabel(row.WalPressure), WalBytes: row.WalBytes,
			PendingEvents: row.PendingEvents, OldestPending: RFC3339NanoPtr(row.OldestPending),
			ClockUntrusted: row.ClockUntrusted, ProtocolVersion: row.ProtocolVersion,
			UpdatedAt: RFC3339TimePtr(row.UpdatedAt), Sinks: []AgentSink{},
		}
		if agent := decoded[i]; agent != nil {
			for _, sink := range agent.GetSinks() {
				kind, name := idx.observer(sink.GetEndpointId())
				record.Sinks = append(record.Sinks, AgentSink{
					EndpointID: sink.GetEndpointId(), ObserverKind: kind, ObserverName: name,
					Connected: sink.GetConnected(), PendingEvents: sink.GetPendingEvents(),
					LastError: sink.GetLastError(), AckThrough: sink.GetAckThrough(),
				})
			}
		}
		out = append(out, record)
	}
	return out, nil
}

func ListIncidents(db *gorm.DB) ([]IncidentRecord, error) {
	out := []IncidentRecord{}
	var rows []model.AvailabilityIncident
	if err := db.Order("started_at DESC").Limit(datasetLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	nodeIDs := make([][]byte, 0, len(rows))
	observerIDs := make([]string, 0)
	evidence := make([][]ObserverEvidenceItem, len(rows))
	for i, row := range rows {
		nodeIDs = append(nodeIDs, row.NodeUUID)
		raw := decodeEvidence(row.ObserverEvidence)
		evidence[i] = raw
		for _, item := range raw {
			observerIDs = append(observerIDs, item.ObserverID)
		}
	}
	idx, err := loadHostIndex(db, uniqueByteSlices(nodeIDs), observerIDs)
	if err != nil {
		return nil, err
	}
	for i, row := range rows {
		serverID, serverName := idx.host(row.NodeUUID)
		out = append(out, IncidentRecord{
			ID: row.ID, ServerID: serverID, ServerName: serverName, NodeUUID: HexUUID(row.NodeUUID),
			InitialClassification: NormalizeLabel(row.InitialClassification),
			CurrentClassification: NormalizeLabel(row.CurrentClassification),
			Revision:              row.Revision, StartedAt: RFC3339NanoPtr(row.StartedAt), EndedAt: RFC3339NanoPtr(row.EndedAt),
			RecalculatedAt: RFC3339NanoPtr(row.RecalculatedAt), Reason: NormalizeLabel(row.Reason),
			ObserverEvidence: annotateEvidence(evidence[i], idx),
		})
	}
	return out, nil
}

func ListIncidentRevisions(db *gorm.DB) ([]IncidentRevisionRecord, error) {
	out := []IncidentRevisionRecord{}
	var rows []model.IncidentRevision
	if err := db.Order("created_at DESC").Limit(datasetLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	observerIDs := make([]string, 0)
	evidence := make([][]ObserverEvidenceItem, len(rows))
	for i, row := range rows {
		raw := decodeEvidence(row.Evidence)
		evidence[i] = raw
		for _, item := range raw {
			observerIDs = append(observerIDs, item.ObserverID)
		}
	}
	idx, err := loadHostIndex(db, nil, observerIDs)
	if err != nil {
		return nil, err
	}
	for i, row := range rows {
		out = append(out, IncidentRevisionRecord{
			ID: row.ID, IncidentID: row.IncidentID, Revision: row.Revision,
			Classification: NormalizeLabel(row.Classification), Reason: NormalizeLabel(row.Reason),
			RecalculatedAt: RFC3339NanoPtr(row.RecalculatedAt), CreatedAt: RFC3339TimePtr(row.CreatedAt),
			ObserverEvidence: annotateEvidence(evidence[i], idx),
		})
	}
	return out, nil
}

func ListDataLoss(db *gorm.DB) ([]DataLossRecord, error) {
	out := []DataLossRecord{}
	var rows []model.TelemetryDataLoss
	if err := db.Order("occurred_at DESC").Limit(datasetLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, DataLossRecord{
			FactID: HexUUID(row.FactID), ComponentID: row.ComponentID, OccurredAt: RFC3339NanoPtr(row.OccurredAt),
			Reason: gapReasonLabel(row.Reason), FirstSpoolID: row.FirstSpoolID, LastSpoolID: row.LastSpoolID,
			LostRecords: row.LostRecords, Detail: row.Detail, CreatedAt: RFC3339TimePtr(row.CreatedAt),
		})
	}
	return out, nil
}

func ListAlerts(db *gorm.DB) ([]AlertRecord, error) {
	out := []AlertRecord{}
	var rows []model.TelemetryAlert
	if err := db.Order("occurred_at DESC").Limit(datasetLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	nodeIDs := make([][]byte, 0, len(rows))
	observerIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row.NodeUUID) > 0 {
			nodeIDs = append(nodeIDs, row.NodeUUID)
		}
		if row.ComponentID != "" {
			observerIDs = append(observerIDs, row.ComponentID)
		}
	}
	idx, err := loadHostIndex(db, uniqueByteSlices(nodeIDs), observerIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		serverID, serverName := idx.host(row.NodeUUID)
		out = append(out, AlertRecord{
			ID: row.ID, AlertType: NormalizeLabel(row.AlertType), Severity: NormalizeLabel(row.Severity),
			ServerID: serverID, ServerName: serverName, NodeUUID: HexUUID(row.NodeUUID),
			ComponentID: row.ComponentID, OccurredAt: RFC3339NanoPtr(row.OccurredAt),
			CreatedAt: RFC3339TimePtr(row.CreatedAt), Notified: row.Notified,
			Message: row.Message, DedupKey: row.DedupKey,
		})
	}
	return out, nil
}

func walPressureLabel(value int32) string {
	switch pb.WalPressure(value) {
	case pb.WalPressure_WAL_PRESSURE_HEALTHY:
		return "healthy"
	case pb.WalPressure_WAL_PRESSURE_P3_DOWNSAMPLED:
		return "downsampled"
	case pb.WalPressure_WAL_PRESSURE_ROLLUP:
		return "rollup"
	case pb.WalPressure_WAL_PRESSURE_CRITICAL:
		return "critical"
	case pb.WalPressure_WAL_PRESSURE_DATA_LOSS:
		return "telemetry_data_loss"
	default:
		return "unknown"
	}
}

func gapReasonLabel(value int32) string {
	switch pb.GapReason(value) {
	case pb.GapReason_GAP_REASON_COMPACTED:
		return "compacted"
	case pb.GapReason_GAP_REASON_HARD_LIMIT_DATA_LOSS:
		return "hard_limit_data_loss"
	case pb.GapReason_GAP_REASON_CORRUPTION:
		return "corruption"
	default:
		return "unknown"
	}
}

type rawEvidence struct {
	ObserverID string `json:"observer_id"`
	Healthy    bool   `json:"healthy"`
	Seen       bool   `json:"seen"`
}

func decodeEvidence(blob []byte) []ObserverEvidenceItem {
	if len(blob) == 0 {
		return []ObserverEvidenceItem{}
	}
	var raw []rawEvidence
	if json.Unmarshal(blob, &raw) != nil {
		return []ObserverEvidenceItem{}
	}
	out := make([]ObserverEvidenceItem, 0, len(raw))
	for _, item := range raw {
		out = append(out, ObserverEvidenceItem{
			ObserverID: item.ObserverID, Healthy: item.Healthy, Seen: item.Seen,
		})
	}
	return out
}

func annotateEvidence(items []ObserverEvidenceItem, idx hostIndex) []ObserverEvidenceItem {
	if items == nil {
		return []ObserverEvidenceItem{}
	}
	out := make([]ObserverEvidenceItem, 0, len(items))
	for _, item := range items {
		kind, name := idx.observer(item.ObserverID)
		item.ObserverKind = kind
		item.ObserverName = name
		out = append(out, item)
	}
	return out
}
