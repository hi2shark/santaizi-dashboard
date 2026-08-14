package telemetry

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RetentionPolicy struct {
	StateRaw       time.Duration
	StateOneMinute time.Duration
	StateOneHour   time.Duration
	Observation    time.Duration
	Lifecycle      time.Duration
	BatchSize      int
}

type RollupWorker struct {
	db        *gorm.DB
	retention RetentionPolicy
}

func NewRollupWorker(db *gorm.DB, policy RetentionPolicy) *RollupWorker {
	if policy.StateRaw <= 0 {
		policy.StateRaw = 6 * time.Hour
	}
	if policy.StateOneMinute <= 0 {
		policy.StateOneMinute = 30 * 24 * time.Hour
	}
	if policy.StateOneHour <= 0 {
		policy.StateOneHour = 365 * 24 * time.Hour
	}
	if policy.Observation <= 0 {
		policy.Observation = 30 * 24 * time.Hour
	}
	if policy.Lifecycle <= 0 {
		policy.Lifecycle = 10 * 365 * 24 * time.Hour
	}
	if policy.BatchSize <= 0 {
		policy.BatchSize = 1000
	}
	return &RollupWorker{db: db, retention: policy}
}

func (w *RollupWorker) Run(ctx context.Context) {
	rollupTicker := time.NewTicker(time.Minute)
	retentionTicker := time.NewTicker(time.Hour)
	defer rollupTicker.Stop()
	defer retentionTicker.Stop()
	_ = w.RollupPending(ctx, time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-rollupTicker.C:
			_ = w.RollupPending(ctx, now)
		case now := <-retentionTicker.C:
			_ = w.ApplyRetention(ctx, now)
		}
	}
}

func (w *RollupWorker) RollupPending(ctx context.Context, now time.Time) error {
	minuteEnd := now.Truncate(time.Minute)
	if err := w.rollupRawWindow(ctx, minuteEnd.Add(-time.Minute), minuteEnd); err != nil {
		return err
	}
	hourEnd := now.Truncate(time.Hour)
	return w.rollupHourWindow(ctx, hourEnd.Add(-time.Hour), hourEnd)
}

func (w *RollupWorker) rollupRawWindow(ctx context.Context, start, end time.Time) error {
	var nodes [][]byte
	if err := w.db.WithContext(ctx).Model(&model.TelemetryEvent{}).
		Where("event_type = ? AND collected_at >= ? AND collected_at < ? AND payload_retained = ?", pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_STATE, start.UnixNano(), end.UnixNano(), true).
		Distinct().Pluck("node_uuid", &nodes).Error; err != nil {
		return err
	}
	for _, node := range nodes {
		var events []model.TelemetryEvent
		if err := w.db.WithContext(ctx).Where("node_uuid = ? AND event_type = ? AND collected_at >= ? AND collected_at < ? AND payload_retained = ?",
			node, pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_STATE, start.UnixNano(), end.UnixNano(), true).Order("collected_at ASC").Find(&events).Error; err != nil {
			return err
		}
		states := make([]*pb.State, 0, len(events))
		for _, row := range events {
			event := new(pb.TelemetryEvent)
			if err := proto.Unmarshal(row.Payload, event); err != nil {
				return err
			}
			if event.GetState() != nil {
				states = append(states, event.GetState())
			}
		}
		payload := aggregateStates(states, start, end)
		if payload == nil {
			continue
		}
		encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
		if err != nil {
			return err
		}
		row := model.StateRollup{
			NodeUUID: node, Resolution: "1m", WindowStart: start.UnixNano(), WindowEnd: end.UnixNano(),
			SampleCount: payload.GetSampleCount(), Payload: encoded, NetInTotal: payload.GetNetInTotal(), NetOutTotal: payload.GetNetOutTotal(),
		}
		if err := w.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "node_uuid"}, {Name: "resolution"}, {Name: "window_start"}}, UpdateAll: true,
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (w *RollupWorker) rollupHourWindow(ctx context.Context, start, end time.Time) error {
	var nodes [][]byte
	if err := w.db.WithContext(ctx).Model(&model.StateRollup{}).Where("resolution = ? AND window_start >= ? AND window_start < ?", "1m", start.UnixNano(), end.UnixNano()).Distinct().Pluck("node_uuid", &nodes).Error; err != nil {
		return err
	}
	for _, node := range nodes {
		var rows []model.StateRollup
		if err := w.db.WithContext(ctx).Where("node_uuid = ? AND resolution = ? AND window_start >= ? AND window_start < ?", node, "1m", start.UnixNano(), end.UnixNano()).Order("window_start ASC").Find(&rows).Error; err != nil {
			return err
		}
		payload := aggregateRollups(rows, start, end)
		if payload == nil {
			continue
		}
		encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
		if err != nil {
			return err
		}
		result := model.StateRollup{
			NodeUUID: node, Resolution: "1h", WindowStart: start.UnixNano(), WindowEnd: end.UnixNano(),
			SampleCount: payload.GetSampleCount(), Payload: encoded, NetInTotal: payload.GetNetInTotal(), NetOutTotal: payload.GetNetOutTotal(),
		}
		if err := w.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "node_uuid"}, {Name: "resolution"}, {Name: "window_start"}}, UpdateAll: true,
		}).Create(&result).Error; err != nil {
			return err
		}
	}
	return nil
}

func aggregateStates(states []*pb.State, start, end time.Time) *pb.StateRollupPayload {
	if len(states) == 0 {
		return nil
	}
	minimum := cloneState(states[0])
	maximum := cloneState(states[0])
	average := new(pb.State)
	for _, state := range states {
		minimum.Cpu = math.Min(minimum.GetCpu(), state.GetCpu())
		minimum.MemUsed = min(minimum.GetMemUsed(), state.GetMemUsed())
		minimum.SwapUsed = min(minimum.GetSwapUsed(), state.GetSwapUsed())
		minimum.DiskUsed = min(minimum.GetDiskUsed(), state.GetDiskUsed())
		minimum.Load1 = math.Min(minimum.GetLoad1(), state.GetLoad1())
		minimum.Load5 = math.Min(minimum.GetLoad5(), state.GetLoad5())
		minimum.Load15 = math.Min(minimum.GetLoad15(), state.GetLoad15())
		minimum.TcpConnCount = min(minimum.GetTcpConnCount(), state.GetTcpConnCount())
		minimum.UdpConnCount = min(minimum.GetUdpConnCount(), state.GetUdpConnCount())
		minimum.ProcessCount = min(minimum.GetProcessCount(), state.GetProcessCount())
		minimum.NetInSpeed = min(minimum.GetNetInSpeed(), state.GetNetInSpeed())
		minimum.NetOutSpeed = min(minimum.GetNetOutSpeed(), state.GetNetOutSpeed())
		maximum.Cpu = math.Max(maximum.GetCpu(), state.GetCpu())
		maximum.MemUsed = max(maximum.GetMemUsed(), state.GetMemUsed())
		maximum.SwapUsed = max(maximum.GetSwapUsed(), state.GetSwapUsed())
		maximum.DiskUsed = max(maximum.GetDiskUsed(), state.GetDiskUsed())
		maximum.Load1 = math.Max(maximum.GetLoad1(), state.GetLoad1())
		maximum.Load5 = math.Max(maximum.GetLoad5(), state.GetLoad5())
		maximum.Load15 = math.Max(maximum.GetLoad15(), state.GetLoad15())
		maximum.TcpConnCount = max(maximum.GetTcpConnCount(), state.GetTcpConnCount())
		maximum.UdpConnCount = max(maximum.GetUdpConnCount(), state.GetUdpConnCount())
		maximum.ProcessCount = max(maximum.GetProcessCount(), state.GetProcessCount())
		maximum.NetInSpeed = max(maximum.GetNetInSpeed(), state.GetNetInSpeed())
		maximum.NetOutSpeed = max(maximum.GetNetOutSpeed(), state.GetNetOutSpeed())
		average.Cpu += state.GetCpu()
		average.MemUsed += state.GetMemUsed()
		average.SwapUsed += state.GetSwapUsed()
		average.DiskUsed += state.GetDiskUsed()
		average.Load1 += state.GetLoad1()
		average.Load5 += state.GetLoad5()
		average.Load15 += state.GetLoad15()
		average.TcpConnCount += state.GetTcpConnCount()
		average.UdpConnCount += state.GetUdpConnCount()
		average.ProcessCount += state.GetProcessCount()
		average.NetInSpeed += state.GetNetInSpeed()
		average.NetOutSpeed += state.GetNetOutSpeed()
	}
	count := uint64(len(states))
	average.Cpu /= float64(count)
	average.MemUsed /= count
	average.SwapUsed /= count
	average.DiskUsed /= count
	average.Load1 /= float64(count)
	average.Load5 /= float64(count)
	average.Load15 /= float64(count)
	average.TcpConnCount /= count
	average.UdpConnCount /= count
	average.ProcessCount /= count
	average.NetInSpeed /= count
	average.NetOutSpeed /= count
	netIn, netOut := counterDeltas(states)
	return &pb.StateRollupPayload{
		WindowStartUnixNano: start.UnixNano(), WindowEndUnixNano: end.UnixNano(), SampleCount: uint32(len(states)),
		Minimum: minimum, Average: average, Maximum: maximum, NetInTotal: netIn, NetOutTotal: netOut,
	}
}

func aggregateRollups(rows []model.StateRollup, start, end time.Time) *pb.StateRollupPayload {
	var states []*pb.State
	var netIn, netOut uint64
	var minimum, maximum *pb.State
	for _, row := range rows {
		payload := new(pb.StateRollupPayload)
		if proto.Unmarshal(row.Payload, payload) != nil || payload.GetAverage() == nil {
			continue
		}
		if payload.GetMinimum() != nil && payload.GetMaximum() != nil {
			if minimum == nil {
				minimum = cloneState(payload.GetMinimum())
				maximum = cloneState(payload.GetMaximum())
			} else {
				mergeStateExtrema(minimum, maximum, payload.GetMinimum(), payload.GetMaximum())
			}
		}
		for index := uint32(0); index < payload.GetSampleCount(); index++ {
			states = append(states, payload.GetAverage())
		}
		netIn += payload.GetNetInTotal()
		netOut += payload.GetNetOutTotal()
	}
	result := aggregateStates(states, start, end)
	if result != nil {
		if minimum != nil {
			result.Minimum = minimum
			result.Maximum = maximum
		}
		result.NetInTotal = netIn
		result.NetOutTotal = netOut
	}
	return result
}

func mergeStateExtrema(minimum, maximum, candidateMinimum, candidateMaximum *pb.State) {
	minimum.Cpu = math.Min(minimum.GetCpu(), candidateMinimum.GetCpu())
	minimum.MemUsed = min(minimum.GetMemUsed(), candidateMinimum.GetMemUsed())
	minimum.SwapUsed = min(minimum.GetSwapUsed(), candidateMinimum.GetSwapUsed())
	minimum.DiskUsed = min(minimum.GetDiskUsed(), candidateMinimum.GetDiskUsed())
	minimum.Load1 = math.Min(minimum.GetLoad1(), candidateMinimum.GetLoad1())
	minimum.Load5 = math.Min(minimum.GetLoad5(), candidateMinimum.GetLoad5())
	minimum.Load15 = math.Min(minimum.GetLoad15(), candidateMinimum.GetLoad15())
	minimum.TcpConnCount = min(minimum.GetTcpConnCount(), candidateMinimum.GetTcpConnCount())
	minimum.UdpConnCount = min(minimum.GetUdpConnCount(), candidateMinimum.GetUdpConnCount())
	minimum.ProcessCount = min(minimum.GetProcessCount(), candidateMinimum.GetProcessCount())
	minimum.NetInSpeed = min(minimum.GetNetInSpeed(), candidateMinimum.GetNetInSpeed())
	minimum.NetOutSpeed = min(minimum.GetNetOutSpeed(), candidateMinimum.GetNetOutSpeed())
	maximum.Cpu = math.Max(maximum.GetCpu(), candidateMaximum.GetCpu())
	maximum.MemUsed = max(maximum.GetMemUsed(), candidateMaximum.GetMemUsed())
	maximum.SwapUsed = max(maximum.GetSwapUsed(), candidateMaximum.GetSwapUsed())
	maximum.DiskUsed = max(maximum.GetDiskUsed(), candidateMaximum.GetDiskUsed())
	maximum.Load1 = math.Max(maximum.GetLoad1(), candidateMaximum.GetLoad1())
	maximum.Load5 = math.Max(maximum.GetLoad5(), candidateMaximum.GetLoad5())
	maximum.Load15 = math.Max(maximum.GetLoad15(), candidateMaximum.GetLoad15())
	maximum.TcpConnCount = max(maximum.GetTcpConnCount(), candidateMaximum.GetTcpConnCount())
	maximum.UdpConnCount = max(maximum.GetUdpConnCount(), candidateMaximum.GetUdpConnCount())
	maximum.ProcessCount = max(maximum.GetProcessCount(), candidateMaximum.GetProcessCount())
	maximum.NetInSpeed = max(maximum.GetNetInSpeed(), candidateMaximum.GetNetInSpeed())
	maximum.NetOutSpeed = max(maximum.GetNetOutSpeed(), candidateMaximum.GetNetOutSpeed())
}

func counterDeltas(states []*pb.State) (uint64, uint64) {
	var inbound, outbound uint64
	for index := 1; index < len(states); index++ {
		previous, current := states[index-1], states[index]
		continuousBoot := current.GetUptime() >= previous.GetUptime()
		inbound += safeCounterDelta(previous.GetNetInTransfer(), current.GetNetInTransfer(), continuousBoot)
		outbound += safeCounterDelta(previous.GetNetOutTransfer(), current.GetNetOutTransfer(), continuousBoot)
	}
	return inbound, outbound
}

func safeCounterDelta(previous, current uint64, continuousBoot bool) uint64 {
	const maximumPlausibleDelta = uint64(1 << 50) // 1 PiB between adjacent samples is treated as corrupt evidence.
	if !continuousBoot {
		return 0
	}
	if current >= previous {
		delta := current - previous
		if delta > maximumPlausibleDelta {
			return 0
		}
		return delta
	}
	if previous > math.MaxUint64-(1<<32) {
		delta := math.MaxUint64 - previous + current + 1
		if delta <= maximumPlausibleDelta {
			return delta
		}
	}
	return 0
}

func cloneState(state *pb.State) *pb.State {
	return proto.Clone(state).(*pb.State)
}

func (w *RollupWorker) ApplyRetention(ctx context.Context, now time.Time) error {
	batch := w.retention.BatchSize
	statePayloadBefore := now.Add(-w.retention.StateRaw).UnixNano()
	if err := w.db.WithContext(ctx).Exec(`UPDATE telemetry_events SET payload = NULL, payload_retained = 0
		WHERE rowid IN (SELECT e.rowid FROM telemetry_events e
		WHERE e.event_type = ? AND e.payload_retained = 1 AND e.collected_at < ?
		AND EXISTS (SELECT 1 FROM state_rollups r WHERE r.node_uuid = e.node_uuid AND r.resolution = '1m'
		AND r.window_start <= e.collected_at AND r.window_end > e.collected_at) LIMIT ?)`,
		pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_STATE, statePayloadBefore, batch).Error; err != nil {
		return err
	}
	observationBefore := now.Add(-w.retention.Observation).UnixNano()
	if err := deleteBatch(w.db.WithContext(ctx), "telemetry_observations", "received_at < ?", observationBefore, batch); err != nil {
		return err
	}
	if err := w.db.WithContext(ctx).Exec(`DELETE FROM telemetry_events WHERE rowid IN
		(SELECT rowid FROM telemetry_events WHERE collected_at < ? AND event_type != ? LIMIT ?)`,
		observationBefore, pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_LIFECYCLE, batch).Error; err != nil {
		return err
	}
	lifecycleBefore := now.Add(-w.retention.Lifecycle).UnixNano()
	for table, column := range map[string]string{
		"telemetry_events": "collected_at", "telemetry_gaps": "created_at_unix_nano",
		"availability_buckets": "bucket_start", "availability_incidents": "started_at",
	} {
		condition := column + " < ?"
		if table == "telemetry_events" {
			condition += " AND event_type = " + itoa(int(pb.TelemetryEventType_TELEMETRY_EVENT_TYPE_LIFECYCLE))
		}
		if err := deleteBatch(w.db.WithContext(ctx), table, condition, lifecycleBefore, batch); err != nil {
			return err
		}
	}
	if err := deleteBatch(w.db.WithContext(ctx), "state_rollups", "resolution = '1m' AND window_start < ?", now.Add(-w.retention.StateOneMinute).UnixNano(), batch); err != nil {
		return err
	}
	if err := deleteBatch(w.db.WithContext(ctx), "state_rollups", "resolution = '1h' AND window_start < ?", now.Add(-w.retention.StateOneHour).UnixNano(), batch); err != nil {
		return err
	}
	return deleteBatch(w.db.WithContext(ctx), "connection_latency_buckets", "bucket_start < ?", now.Add(-ConnectionLatencyRetention).UnixNano(), batch)
}

func deleteBatch(db *gorm.DB, table, condition string, before int64, limit int) error {
	allowed := map[string]bool{
		"telemetry_observations": true, "telemetry_events": true, "telemetry_gaps": true,
		"availability_buckets": true, "availability_incidents": true, "state_rollups": true,
		"connection_latency_buckets": true,
	}
	if !allowed[table] {
		return errors.New("retention table is not allowlisted")
	}
	return db.Exec("DELETE FROM "+table+" WHERE rowid IN (SELECT rowid FROM "+table+" WHERE "+condition+" LIMIT ?)", before, limit).Error
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
