package telemetry

import (
	"bytes"
	"encoding/hex"
	"errors"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	LatencyKindCollectorHeartbeat   = "collector_heartbeat"
	LatencyKindCollectorReplication = "collector_replication"
	LatencyKindPath                 = "path"
	ConnectionLatencyRetention      = 24 * time.Hour
	latencyBucketSize               = int64(time.Minute)
)

var zeroNodeUUID [16]byte

type LatencySample struct {
	Kind          string
	CollectorUUID string
	NodeUUID      []byte
	ObserverID    string
	RttMs         float64
	SampledAt     int64
}

type LatencyFilter struct {
	Kind          string
	CollectorUUID string
	ServerID      uint64
	ObserverID    string
}

type LatencyBucket struct {
	Kind          string
	CollectorUUID string
	ServerID      uint64
	ServerName    string
	NodeUUID      string
	ObserverID    string
	BucketStart   int64
	MinMs         float64
	AvgMs         float64
	MaxMs         float64
	Count         uint32
}

func RecordCollectorLatency(db *gorm.DB, row model.CollectorRuntime) error {
	if err := RecordLatencySample(db, LatencySample{
		Kind: LatencyKindCollectorHeartbeat, CollectorUUID: row.CollectorUUID,
		RttMs: row.HeartbeatRttMs, SampledAt: row.HeartbeatRttSampledAt,
	}); err != nil {
		return err
	}
	return RecordLatencySample(db, LatencySample{
		Kind: LatencyKindCollectorReplication, CollectorUUID: row.CollectorUUID,
		RttMs: row.ReplicationRttMs, SampledAt: row.ReplicationRttSampledAt,
	})
}

func RecordAgentSinkLatency(db *gorm.DB, nodeUUID []byte, runtime *pb.AgentRuntime) error {
	if runtime == nil {
		return nil
	}
	for _, sink := range runtime.GetSinks() {
		if err := RecordLatencySample(db, LatencySample{
			Kind: LatencyKindPath, NodeUUID: nodeUUID, ObserverID: sink.GetEndpointId(),
			RttMs: sink.GetLastRttMs(), SampledAt: sink.GetRttSampledAtUnixNano(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func RecordLatencySample(db *gorm.DB, sample LatencySample) error {
	if sample.SampledAt <= 0 || sample.RttMs < 0 || sample.Kind == "" {
		return nil
	}
	if !validLatencyKind(sample.Kind) {
		return errors.New("unknown connection latency kind")
	}
	sample.NodeUUID = latencyNodeKey(sample.NodeUUID)
	return db.Transaction(func(tx *gorm.DB) error {
		var cursor model.ConnectionLatencyCursor
		if err := tx.Where("kind = ? AND collector_uuid = ? AND node_uuid = ? AND observer_id = ?",
			sample.Kind, sample.CollectorUUID, sample.NodeUUID, sample.ObserverID).Limit(1).Find(&cursor).Error; err != nil {
			return err
		}
		if cursor.LastSampledAt >= sample.SampledAt {
			return nil
		}
		bucketStart := sample.SampledAt / latencyBucketSize * latencyBucketSize
		bucket := model.ConnectionLatencyBucket{
			Kind: sample.Kind, CollectorUUID: sample.CollectorUUID, NodeUUID: sample.NodeUUID,
			ObserverID: sample.ObserverID, BucketStart: bucketStart,
			MinMs: sample.RttMs, MaxMs: sample.RttMs, SumMs: sample.RttMs, Count: 1,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "kind"}, {Name: "collector_uuid"}, {Name: "node_uuid"}, {Name: "observer_id"}, {Name: "bucket_start"}},
			DoUpdates: clause.Assignments(map[string]any{
				"min_ms":     gorm.Expr("MIN(min_ms, excluded.min_ms)"),
				"max_ms":     gorm.Expr("MAX(max_ms, excluded.max_ms)"),
				"sum_ms":     gorm.Expr("sum_ms + excluded.sum_ms"),
				"count":      gorm.Expr("count + excluded.count"),
				"updated_at": time.Now(),
			}),
		}).Create(&bucket).Error; err != nil {
			return err
		}
		cursor = model.ConnectionLatencyCursor{
			Kind: sample.Kind, CollectorUUID: sample.CollectorUUID, NodeUUID: sample.NodeUUID,
			ObserverID: sample.ObserverID, LastSampledAt: sample.SampledAt,
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "kind"}, {Name: "collector_uuid"}, {Name: "node_uuid"}, {Name: "observer_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"last_sampled_at", "updated_at"}),
		}).Create(&cursor).Error
	})
}

func ListConnectionLatency(db *gorm.DB, filter LatencyFilter, offset, limit int) ([]LatencyBucket, int64, error) {
	if filter.Kind != "" && !validLatencyKind(filter.Kind) {
		return nil, 0, errors.New("unknown connection latency kind")
	}
	query := db.Model(&model.ConnectionLatencyBucket{})
	if filter.Kind != "" {
		query = query.Where("kind = ?", filter.Kind)
	}
	if filter.CollectorUUID != "" {
		query = query.Where("collector_uuid = ?", filter.CollectorUUID)
	}
	if filter.ObserverID != "" {
		query = query.Where("observer_id = ?", filter.ObserverID)
	}
	if filter.ServerID > 0 {
		var binding model.ServerNodeBinding
		if err := db.Where("server_id = ? AND current = ?", filter.ServerID, true).Limit(1).Find(&binding).Error; err != nil {
			return nil, 0, err
		}
		if binding.ServerID == 0 {
			return []LatencyBucket{}, 0, nil
		}
		query = query.Where("node_uuid = ?", binding.NodeUUID)
	}
	rows, total, err := listPage[model.ConnectionLatencyBucket](query, "bucket_start DESC", offset, limit)
	if err != nil {
		return nil, 0, err
	}
	nodeIDs := make([][]byte, 0, len(rows))
	observerIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if !bytes.Equal(row.NodeUUID, zeroNodeUUID[:]) {
			nodeIDs = append(nodeIDs, row.NodeUUID)
		}
		if row.ObserverID != "" {
			observerIDs = append(observerIDs, row.ObserverID)
		}
	}
	idx, err := loadHostIndex(db, uniqueByteSlices(nodeIDs), observerIDs)
	if err != nil {
		return nil, 0, err
	}
	out := make([]LatencyBucket, 0, len(rows))
	for _, row := range rows {
		serverID, serverName := idx.host(row.NodeUUID)
		avg := 0.0
		if row.Count > 0 {
			avg = row.SumMs / float64(row.Count)
		}
		out = append(out, LatencyBucket{
			Kind: row.Kind, CollectorUUID: row.CollectorUUID, ServerID: serverID, ServerName: serverName,
			NodeUUID: hexLatencyNode(row.NodeUUID), ObserverID: row.ObserverID, BucketStart: row.BucketStart,
			MinMs: row.MinMs, AvgMs: avg, MaxMs: row.MaxMs, Count: row.Count,
		})
	}
	return out, total, nil
}

func validLatencyKind(kind string) bool {
	switch kind {
	case LatencyKindCollectorHeartbeat, LatencyKindCollectorReplication, LatencyKindPath:
		return true
	default:
		return false
	}
}

func latencyNodeKey(value []byte) []byte {
	if len(value) != 16 {
		return append([]byte(nil), zeroNodeUUID[:]...)
	}
	return append([]byte(nil), value...)
}

func hexLatencyNode(value []byte) string {
	if len(value) == 0 || bytes.Equal(value, zeroNodeUUID[:]) {
		return ""
	}
	return hex.EncodeToString(value)
}
