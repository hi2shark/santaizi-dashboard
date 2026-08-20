package telemetry

import (
	"context"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const availabilityHourNs = int64(time.Hour)

type availabilityHourKey struct {
	NodeUUID  []byte `gorm:"column:node_uuid"`
	HourStart int64  `gorm:"column:hour_start"`
}

func compactAvailabilitySpans(ctx context.Context, db *gorm.DB, deadline time.Time, limit int, cutoff int64) (int64, error) {
	if !sqliteTableExists(db, "availability_buckets") {
		return 0, nil
	}
	if limit <= 0 {
		limit = DefaultRetentionBatch
	}
	hourCutoff := cutoff / availabilityHourNs * availabilityHourNs
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			return total, nil
		}
		var hours []availabilityHourKey
		if err := db.WithContext(ctx).Raw(
			`SELECT node_uuid, (bucket_start / ?) * ? AS hour_start
			FROM availability_buckets
			WHERE (resolution = '' OR resolution = ?) AND bucket_start < ?
			GROUP BY node_uuid, hour_start
			ORDER BY hour_start ASC
			LIMIT ?`,
			availabilityHourNs, availabilityHourNs, model.AvailabilityResolutionRaw, hourCutoff, limit,
		).Scan(&hours).Error; err != nil {
			return total, err
		}
		if len(hours) == 0 {
			return total, nil
		}
		progress := false
		for _, hour := range hours {
			n, err := compactAvailabilityHour(ctx, db, hour.NodeUUID, hour.HourStart)
			if err != nil {
				return total, err
			}
			if n > 0 {
				progress = true
				total += n
			}
		}
		if !progress {
			return total, nil
		}
	}
}

func compactAvailabilityHour(ctx context.Context, db *gorm.DB, nodeUUID []byte, hourStart int64) (int64, error) {
	hourEnd := hourStart + availabilityHourNs
	var rows []model.AvailabilityBucket
	if err := db.WithContext(ctx).Where(
		"node_uuid = ? AND bucket_start >= ? AND bucket_start < ? AND (resolution = '' OR resolution = ?)",
		nodeUUID, hourStart, hourEnd, model.AvailabilityResolutionRaw,
	).Order("bucket_start ASC").Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	spans := mergeAvailabilityRuns(rows)
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"node_uuid = ? AND bucket_start >= ? AND bucket_start < ? AND (resolution = '' OR resolution = ?)",
			nodeUUID, hourStart, hourEnd, model.AvailabilityResolutionRaw,
		).Delete(&model.AvailabilityBucket{}).Error; err != nil {
			return err
		}
		for i := range spans {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "node_uuid"}, {Name: "bucket_start"}}, UpdateAll: true,
			}).Create(&spans[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

func mergeAvailabilityRuns(rows []model.AvailabilityBucket) []model.AvailabilityBucket {
	if len(rows) == 0 {
		return nil
	}
	spans := make([]model.AvailabilityBucket, 0, len(rows))
	current := startAvailabilitySpan(rows[0])
	for _, row := range rows[1:] {
		end := availabilityRowEnd(row)
		if sameAvailabilityRun(current, row) && row.BucketStart == current.WindowEnd {
			current.WindowEnd = end
			if row.Revision > current.Revision {
				current.Revision = row.Revision
			}
			current.ObserverSummary = row.ObserverSummary
			current.Finalized = row.Finalized
			current.RecalculatedAt = row.RecalculatedAt
			continue
		}
		spans = append(spans, current)
		current = startAvailabilitySpan(row)
	}
	return append(spans, current)
}

func startAvailabilitySpan(row model.AvailabilityBucket) model.AvailabilityBucket {
	span := row
	span.Resolution = model.AvailabilityResolutionSpan
	span.WindowEnd = availabilityRowEnd(row)
	return span
}

func availabilityRowEnd(row model.AvailabilityBucket) int64 {
	if row.WindowEnd > row.BucketStart {
		return row.WindowEnd
	}
	return row.BucketStart + int64(30*time.Second)
}

func sameAvailabilityRun(span, row model.AvailabilityBucket) bool {
	return span.HostState == row.HostState &&
		span.ConnectivityState == row.ConnectivityState &&
		span.ExpectedObservers == row.ExpectedObservers &&
		span.HealthyObservers == row.HealthyObservers &&
		span.SeenObservers == row.SeenObservers
}
