package collector

import (
	"testing"
	"time"
)

func TestNoteSyncRTTRecordsHeartbeatSample(t *testing.T) {
	runtime := &Runtime{}
	now := time.Unix(1_800_000_000, 0)
	runtime.markSyncSent(now.Add(-20 * time.Millisecond))
	runtime.noteSyncRTT(now)
	if runtime.heartbeatRttMs < 19 || runtime.heartbeatRttMs > 21 || runtime.heartbeatRttAt != now.UnixNano() || !runtime.syncSentAt.IsZero() {
		t.Fatalf("heartbeat rtt=%v sampled=%d sent=%v", runtime.heartbeatRttMs, runtime.heartbeatRttAt, runtime.syncSentAt)
	}
	runtime.noteSyncRTT(now.Add(time.Second))
	if runtime.heartbeatRttMs < 19 || runtime.heartbeatRttMs > 21 {
		t.Fatalf("empty outstanding send should not overwrite, got %v", runtime.heartbeatRttMs)
	}
}

func TestNoteReplicationRTTRecordsSample(t *testing.T) {
	runtime := &Runtime{}
	now := time.Unix(1_800_000_000, 0)
	runtime.noteReplicationRTT(now.Add(-8*time.Millisecond), now)
	if runtime.replicationRttMs < 7.5 || runtime.replicationRttMs > 8.5 || runtime.replicationRttAt != now.UnixNano() {
		t.Fatalf("replication rtt=%v sampled=%d", runtime.replicationRttMs, runtime.replicationRttAt)
	}
}
