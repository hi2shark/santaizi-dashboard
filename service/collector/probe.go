package collector

import (
	"context"
	"time"

	"github.com/hi2shark/santaizi-dashboard/model"
	"github.com/hi2shark/santaizi-dashboard/pkg/netprobe"
	pb "github.com/hi2shark/santaizi-dashboard/proto"
)

type probeState struct {
	lastCycle    time.Time
	lastMTR      time.Time
	lastReachable *bool
}

func (r *Runtime) isProbe() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.kind == model.CollectorKindProbe
}

func (r *Runtime) setKind(kind string) {
	r.mu.Lock()
	r.kind = model.NormalizeCollectorKind(kind)
	r.mu.Unlock()
}

func (r *Runtime) probeLoop() {
	defer r.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	states := map[uint64]*probeState{}
	for {
		select {
		case <-r.ctx.Done():
			return
		case now := <-ticker.C:
			if !r.isProbe() {
				continue
			}
			targets, err := r.store.ProbeTargets(r.ctx)
			if err != nil || len(targets) == 0 {
				continue
			}
			var batch []*pb.ProbeSample
			for _, target := range targets {
				state := states[target.ServerID]
				if state == nil {
					state = &probeState{}
					states[target.ServerID] = state
				}
				interval := time.Duration(target.IntervalSec) * time.Second
				if interval <= 0 {
					interval = 30 * time.Second
				}
				if !state.lastCycle.IsZero() && now.Sub(state.lastCycle) < interval {
					continue
				}
				sample := r.runProbeTarget(r.ctx, target, state, now)
				state.lastCycle = now
				if sample != nil {
					batch = append(batch, sample)
				}
			}
			if len(batch) > 0 {
				r.queueProbeSamples(batch)
			}
		}
	}
}

func (r *Runtime) runProbeTarget(ctx context.Context, target model.CollectorCachedProbeTarget, state *probeState, now time.Time) *pb.ProbeSample {
	host := netprobe.FormatHost(target.IPv4, target.IPv6, target.Hostname)
	if host == "" {
		return nil
	}
	sample := &pb.ProbeSample{ServerId: target.ServerID, SampledAtUnixNano: now.UnixNano()}
	var icmp netprobe.ICMPResult
	if target.EnableICMP {
		icmp = netprobe.ICMP(ctx, host, 5, 2*time.Second)
		sample.Icmp = &pb.ProbeICMPSample{
			Ok: icmp.OK, RttMs: durationMilliseconds(icmp.RTT), Loss: icmp.Loss,
			PacketsSent: uint32(icmp.Sent), PacketsReceived: uint32(icmp.Received), Error: icmp.Error,
		}
	}
	var tcpResults []netprobe.TCPResult
	if target.EnableTCP {
		for _, port := range netprobe.ParsePorts(target.TCPPorts) {
			result := netprobe.TCP(ctx, host, port, 3*time.Second)
			tcpResults = append(tcpResults, result)
			sample.Tcp = append(sample.Tcp, &pb.ProbeTCPSample{
				Port: uint32(result.Port), Ok: result.OK, RttMs: durationMilliseconds(result.RTT), Error: result.Error,
			})
		}
	}
	reachable := icmp.OK
	for _, item := range tcpResults {
		if item.OK {
			reachable = true
			break
		}
	}
	sample.LastError = netprobe.DisplayError(icmp, tcpResults)
	flipped := state.lastReachable != nil && *state.lastReachable != reachable
	state.lastReachable = &reachable
	mtrEvery := time.Duration(target.MTRIntervalSec) * time.Second
	if mtrEvery <= 0 {
		mtrEvery = 5 * time.Minute
	}
	if target.EnableMTR && (flipped || state.lastMTR.IsZero() || now.Sub(state.lastMTR) >= mtrEvery) {
		trace := netprobe.MTR(ctx, host, 30, 3, time.Second)
		state.lastMTR = now
		pbTrace := &pb.ProbeMTRTrace{SampledAtUnixNano: now.UnixNano(), Destination: trace.Destination}
		for _, hop := range trace.Hops {
			pbTrace.Hops = append(pbTrace.Hops, &pb.ProbeMTRHop{
				Ttl: uint32(hop.TTL), Address: hop.Address, Loss: hop.Loss,
				AvgMs: durationMilliseconds(hop.Avg), Sent: uint32(hop.Sent),
			})
		}
		sample.Mtr = pbTrace
	}
	return sample
}

func (r *Runtime) queueProbeSamples(samples []*pb.ProbeSample) {
	r.probeMu.Lock()
	r.pendingSamples = append(r.pendingSamples, samples...)
	if len(r.pendingSamples) > 256 {
		r.pendingSamples = r.pendingSamples[len(r.pendingSamples)-256:]
	}
	r.probeMu.Unlock()
}

func (r *Runtime) takeProbeSamples() *pb.ProbeSampleBatch {
	r.probeMu.Lock()
	defer r.probeMu.Unlock()
	if len(r.pendingSamples) == 0 {
		return nil
	}
	batch := &pb.ProbeSampleBatch{Samples: r.pendingSamples}
	r.pendingSamples = nil
	return batch
}
