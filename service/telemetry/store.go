package telemetry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const clockSkewLimit = 5 * time.Minute

type Store struct {
	db         *gorm.DB
	bucketSize int64
}

func NewStore(db *gorm.DB) *Store {
	return NewStoreWithBucketSize(db, 30*time.Second)
}

func NewStoreWithBucketSize(db *gorm.DB, bucketSize time.Duration) *Store {
	if bucketSize <= 0 {
		bucketSize = 30 * time.Second
	}
	return &Store{db: db, bucketSize: int64(bucketSize)}
}

func EventID(nodeUUID, sessionID []byte, sequence uint64) ([]byte, error) {
	if len(nodeUUID) != 16 || len(sessionID) != 16 {
		return nil, errors.New("node UUID and session ID must be 16 bytes")
	}
	var input [40]byte
	copy(input[:16], nodeUUID)
	copy(input[16:32], sessionID)
	binary.BigEndian.PutUint64(input[32:], sequence)
	sum := sha256.Sum256(input[:])
	return append([]byte(nil), sum[:16]...), nil
}

type IngestResult struct {
	Acks        []*pb.SessionAck
	FreshEvents []*pb.TelemetryEvent
}

func (s *Store) Replicate(ctx context.Context, batch *pb.ReplicationBatch, receivedAt time.Time) (uint64, error) {
	if batch == nil || batch.GetCollectorUuid() == "" || len(batch.GetReplicationSession()) != 16 || batch.GetBatchSequence() == 0 {
		return 0, errors.New("invalid replication batch identity")
	}
	committed := batch.GetSpoolThroughId()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.CollectorReplicationReceipt
		err := tx.Where("collector_uuid = ? AND replication_session = ? AND batch_sequence = ?",
			batch.GetCollectorUuid(), batch.GetReplicationSession(), batch.GetBatchSequence()).First(&existing).Error
		if err == nil {
			committed = existing.CommittedSpoolThrough
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		for _, event := range batch.GetEvents() {
			if err := validateEvent(event); err != nil {
				return err
			}
			encoded, err := proto.Marshal(event)
			if err != nil {
				return err
			}
			clockUntrusted := absDuration(receivedAt.Sub(time.Unix(0, event.GetCollectedAtUnixNano()))) > clockSkewLimit
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TelemetryEvent{
				EventID: event.GetEventId(), NodeUUID: event.GetNodeUuid(), SessionID: event.GetSessionId(),
				Sequence: event.GetSequence(), EventType: int32(event.GetEventType()), Priority: int32(event.GetPriority()),
				CollectedAt: event.GetCollectedAtUnixNano(), AgentUptimeNano: event.GetAgentUptimeNano(),
				SessionElapsedNano: event.GetSessionElapsedNano(), ClockUntrusted: clockUntrusted,
				SourceProtocol: int32(event.GetSourceProtocol()), Reliability: int32(event.GetReliability()),
				Payload: encoded, PayloadRetained: true,
			}).Error; err != nil {
				return err
			}
		}
		for _, observation := range batch.GetObservations() {
			if len(observation.GetEventId()) != 16 || observation.GetObserverId() == "" {
				return errors.New("invalid replicated observation")
			}
			var event model.TelemetryEvent
			if err := tx.Select("node_uuid", "collected_at", "clock_untrusted").First(&event, "event_id = ?", observation.GetEventId()).Error; err != nil {
				return fmt.Errorf("replicated observation references unknown event: %w", err)
			}
			row := model.TelemetryObservation{
				EventID: observation.GetEventId(), ObserverID: observation.GetObserverId(), NodeUUID: event.NodeUUID,
				ReceivedAt: observation.GetReceivedAtUnixNano(), ReplicatedAt: receivedAt.UnixNano(), Metadata: observation.GetMetadata(),
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				return err
			}
			evidenceAt := event.CollectedAt
			if event.ClockUntrusted {
				evidenceAt = observation.GetReceivedAtUnixNano()
			}
			if err := s.recordPathEvidence(tx, event.NodeUUID, observation.GetObserverId(), evidenceAt, time.Unix(0, observation.GetReceivedAtUnixNano())); err != nil {
				return err
			}
		}
		for _, gap := range batch.GetGaps() {
			if err := validateGap(gap); err != nil {
				return err
			}
			row := model.TelemetryGap{
				GapID: gap.GetGapId(), NodeUUID: gap.GetNodeUuid(), SessionID: gap.GetSessionId(),
				StartSequence: gap.GetStartSequence(), EndSequence: gap.GetEndSequence(), Reason: int32(gap.GetReason()),
				ReplacementEventID: gap.GetReplacementEventId(), CreatedAtUnixNano: gap.GetCreatedAtUnixNano(),
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		for _, health := range batch.GetHealth() {
			bucketSeconds := s.bucketSize
			bucketStart := health.GetSampledAtUnixNano() / bucketSeconds * bucketSeconds
			row := model.ObserverHealthBucket{
				ObserverID: health.GetObserverId(), BucketStart: bucketStart, Healthy: health.GetHealthy(),
				ProcessSession: health.GetProcessSession(), Revision: 1,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "observer_id"}, {Name: "bucket_start"}},
				DoUpdates: clause.AssignmentColumns([]string{"healthy", "process_session", "revision", "updated_at"}),
			}).Create(&row).Error; err != nil {
				return err
			}
			var assignments []model.ObserverAssignment
			if err := tx.Where("observer_id = ? AND valid_from < ? AND (valid_to = 0 OR valid_to > ?)", health.GetObserverId(), bucketStart+bucketSeconds, bucketStart).Find(&assignments).Error; err != nil {
				return err
			}
			for _, assignment := range assignments {
				if err := enqueueAvailability(tx, assignment.NodeUUID, bucketStart, "observer_health"); err != nil {
					return err
				}
			}
		}
		for _, fact := range batch.GetDataLoss() {
			if len(fact.GetFactId()) != 16 || fact.GetCollectorUuid() != batch.GetCollectorUuid() || fact.GetReason() == pb.GapReason_GAP_REASON_UNSPECIFIED {
				return errors.New("invalid collector data loss fact")
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TelemetryDataLoss{
				FactID: fact.GetFactId(), ComponentID: fact.GetCollectorUuid(), OccurredAt: fact.GetOccurredAtUnixNano(),
				Reason: int32(fact.GetReason()), FirstSpoolID: fact.GetFirstSpoolId(), LastSpoolID: fact.GetLastSpoolId(),
				LostRecords: fact.GetLostRecords(), Detail: fact.GetDetail(),
			}).Error; err != nil {
				return err
			}
		}
		if runtime := batch.GetRuntime(); runtime != nil {
			row := CollectorRuntimeFromProto(batch.GetCollectorUuid(), runtime, receivedAt, true)
			if err := UpsertCollectorRuntime(tx, row, true); err != nil {
				return err
			}
		}
		return tx.Create(&model.CollectorReplicationReceipt{
			CollectorUUID: batch.GetCollectorUuid(), ReplicationSession: batch.GetReplicationSession(),
			BatchSequence: batch.GetBatchSequence(), CommittedSpoolThrough: batch.GetSpoolThroughId(),
		}).Error
	})
	return committed, err
}

func (s *Store) recordPathEvidence(tx *gorm.DB, nodeUUID []byte, observerID string, evidenceAt int64, receivedAt time.Time) error {
	bucketStart := evidenceAt / s.bucketSize * s.bucketSize
	health := model.ObserverHealthBucket{ObserverID: observerID, BucketStart: bucketStart, Healthy: true, Revision: 1}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "observer_id"}, {Name: "bucket_start"}},
		DoUpdates: clause.Assignments(map[string]any{"healthy": true, "updated_at": receivedAt}),
	}).Create(&health).Error; err != nil {
		return err
	}
	path := model.ObserverPathBucket{
		NodeUUID: nodeUUID, ObserverID: observerID, BucketStart: bucketStart,
		Seen: true, LastSeenAt: evidenceAt, Revision: 1,
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_uuid"}, {Name: "observer_id"}, {Name: "bucket_start"}},
		DoUpdates: clause.Assignments(map[string]any{
			"seen": true, "last_seen_at": evidenceAt, "updated_at": receivedAt,
		}),
	}).Create(&path).Error; err != nil {
		return err
	}
	return enqueueAvailability(tx, nodeUUID, bucketStart, "observation")
}

func enqueueAvailability(tx *gorm.DB, nodeUUID []byte, bucketStart int64, reason string) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_uuid"}, {Name: "bucket_start"}},
		DoUpdates: clause.Assignments(map[string]any{
			"reason": reason, "updated_at": time.Now(),
		}),
	}).Create(&model.AvailabilityRecomputeQueue{NodeUUID: nodeUUID, BucketStart: bucketStart, Reason: reason}).Error
}

func (s *Store) Ingest(ctx context.Context, batch *pb.TelemetryBatch, observerID string, receivedAt time.Time) (*IngestResult, error) {
	if batch == nil || observerID == "" {
		return nil, errors.New("telemetry batch and observer ID are required")
	}
	result := &IngestResult{}
	maxBySession := make(map[string]uint64)
	sessionIDs := make(map[string][]byte)
	nodeBySession := make(map[string][]byte)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, record := range batch.GetRecords() {
			switch body := record.GetRecord().(type) {
			case *pb.TelemetryRecord_Event:
				event := body.Event
				if err := validateEvent(event); err != nil {
					return err
				}
				encoded, err := proto.Marshal(event)
				if err != nil {
					return err
				}
				clockUntrusted := absDuration(receivedAt.Sub(time.Unix(0, event.GetCollectedAtUnixNano()))) > clockSkewLimit
				row := model.TelemetryEvent{
					EventID:            append([]byte(nil), event.GetEventId()...),
					NodeUUID:           append([]byte(nil), event.GetNodeUuid()...),
					SessionID:          append([]byte(nil), event.GetSessionId()...),
					Sequence:           event.GetSequence(),
					EventType:          int32(event.GetEventType()),
					Priority:           int32(event.GetPriority()),
					CollectedAt:        event.GetCollectedAtUnixNano(),
					AgentUptimeNano:    event.GetAgentUptimeNano(),
					SessionElapsedNano: event.GetSessionElapsedNano(),
					ClockUntrusted:     clockUntrusted,
					SourceProtocol:     int32(event.GetSourceProtocol()),
					Reliability:        int32(event.GetReliability()),
					Payload:            encoded,
					PayloadRetained:    true,
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
					return err
				}
				observation := model.TelemetryObservation{
					EventID:    append([]byte(nil), event.GetEventId()...),
					ObserverID: observerID,
					NodeUUID:   append([]byte(nil), event.GetNodeUuid()...),
					ReceivedAt: receivedAt.UnixNano(),
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&observation).Error; err != nil {
					return err
				}
				evidenceAt := event.GetCollectedAtUnixNano()
				if clockUntrusted {
					evidenceAt = receivedAt.UnixNano()
				}
				if err := s.recordPathEvidence(tx, event.GetNodeUuid(), observerID, evidenceAt, receivedAt); err != nil {
					return err
				}
				key := sessionKey(event.GetNodeUuid(), event.GetSessionId())
				if event.GetSequence() > maxBySession[key] {
					maxBySession[key] = event.GetSequence()
				}
				sessionIDs[key] = append([]byte(nil), event.GetSessionId()...)
				nodeBySession[key] = append([]byte(nil), event.GetNodeUuid()...)
				if !clockUntrusted && receivedAt.Sub(time.Unix(0, event.GetCollectedAtUnixNano())) <= 30*time.Second {
					result.FreshEvents = append(result.FreshEvents, event)
				}
			case *pb.TelemetryRecord_Gap:
				gap := body.Gap
				if err := validateGap(gap); err != nil {
					return err
				}
				row := model.TelemetryGap{
					GapID:              append([]byte(nil), gap.GetGapId()...),
					NodeUUID:           append([]byte(nil), gap.GetNodeUuid()...),
					SessionID:          append([]byte(nil), gap.GetSessionId()...),
					StartSequence:      gap.GetStartSequence(),
					EndSequence:        gap.GetEndSequence(),
					Reason:             int32(gap.GetReason()),
					ReplacementEventID: append([]byte(nil), gap.GetReplacementEventId()...),
					CreatedAtUnixNano:  gap.GetCreatedAtUnixNano(),
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
					return err
				}
				key := sessionKey(gap.GetNodeUuid(), gap.GetSessionId())
				if gap.GetEndSequence() > maxBySession[key] {
					maxBySession[key] = gap.GetEndSequence()
				}
				sessionIDs[key] = append([]byte(nil), gap.GetSessionId()...)
				nodeBySession[key] = append([]byte(nil), gap.GetNodeUuid()...)
			default:
				return errors.New("empty telemetry record")
			}
		}

		keys := make([]string, 0, len(maxBySession))
		for key := range maxBySession {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			ack, err := advanceCursor(tx, observerID, nodeBySession[key], sessionIDs[key], maxBySession[key])
			if err != nil {
				return err
			}
			result.Acks = append(result.Acks, &pb.SessionAck{NodeUuid: nodeBySession[key], SessionId: sessionIDs[key], AckThrough: ack})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func validateEvent(event *pb.TelemetryEvent) error {
	if event == nil || len(event.GetEventId()) != 16 || len(event.GetNodeUuid()) != 16 || len(event.GetSessionId()) != 16 || event.GetSequence() == 0 {
		return errors.New("invalid telemetry event identity")
	}
	expected, err := EventID(event.GetNodeUuid(), event.GetSessionId(), event.GetSequence())
	if err != nil || !bytes.Equal(expected, event.GetEventId()) {
		return errors.New("telemetry event ID does not match identity tuple")
	}
	if event.GetEventType() == pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_UNSPECIFIED || event.GetPriority() == pb.TelemetryPriority_TELEMETRY_PRIORITY_UNSPECIFIED || event.GetPayload() == nil {
		return errors.New("telemetry event type, priority and payload are required")
	}
	return nil
}

func validateGap(gap *pb.SequenceGap) error {
	if gap == nil || len(gap.GetGapId()) != 16 || len(gap.GetNodeUuid()) != 16 || len(gap.GetSessionId()) != 16 || gap.GetStartSequence() == 0 || gap.GetEndSequence() < gap.GetStartSequence() || gap.GetReason() == pb.GapReason_GAP_REASON_UNSPECIFIED {
		return errors.New("invalid telemetry sequence gap")
	}
	return nil
}

func advanceCursor(tx *gorm.DB, receiverID string, nodeUUID, sessionID []byte, maxSequence uint64) (uint64, error) {
	var cursor model.TelemetryIngestCursor
	err := tx.Where("receiver_id = ? AND node_uuid = ? AND session_id = ?", receiverID, nodeUUID, sessionID).First(&cursor).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cursor = model.TelemetryIngestCursor{ReceiverID: receiverID, NodeUUID: append([]byte(nil), nodeUUID...), SessionID: append([]byte(nil), sessionID...)}
	}
	if maxSequence <= cursor.AckThrough {
		return cursor.AckThrough, nil
	}
	var sequences []uint64
	if err := tx.Model(&model.TelemetryEvent{}).
		Where("node_uuid = ? AND session_id = ? AND sequence > ? AND sequence <= ?", nodeUUID, sessionID, cursor.AckThrough, maxSequence).
		Pluck("sequence", &sequences).Error; err != nil {
		return 0, err
	}
	present := make(map[uint64]bool, len(sequences))
	for _, sequence := range sequences {
		present[sequence] = true
	}
	var gaps []model.TelemetryGap
	if err := tx.Where("node_uuid = ? AND session_id = ? AND end_sequence > ? AND start_sequence <= ?", nodeUUID, sessionID, cursor.AckThrough, maxSequence).Find(&gaps).Error; err != nil {
		return 0, err
	}
	for cursor.AckThrough < maxSequence {
		next := cursor.AckThrough + 1
		if present[next] {
			cursor.AckThrough = next
			continue
		}
		advanced := false
		for _, gap := range gaps {
			if gap.StartSequence <= next && gap.EndSequence >= next {
				cursor.AckThrough = gap.EndSequence
				advanced = true
				break
			}
		}
		if !advanced {
			break
		}
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "receiver_id"}, {Name: "node_uuid"}, {Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"ack_through", "updated_at"}),
	}).Create(&cursor).Error; err != nil {
		return 0, err
	}
	return cursor.AckThrough, nil
}

func sessionKey(nodeUUID, sessionID []byte) string {
	return fmt.Sprintf("%x/%x", nodeUUID, sessionID)
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}
