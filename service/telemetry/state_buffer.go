package telemetry

import (
	"encoding/hex"
	"sync"

	pb "github.com/hi2shark/santaizi-dashboard/proto"
	"google.golang.org/protobuf/proto"
)

var sharedStateBuffer *StateSampleBuffer

func SetSharedStateBuffer(buffer *StateSampleBuffer) {
	sharedStateBuffer = buffer
}

func liveStateBuffer() *StateSampleBuffer {
	if sharedStateBuffer != nil {
		return sharedStateBuffer
	}
	return NewStateSampleBuffer()
}

type liveStateSample struct {
	eventID     string
	nodeUUID    []byte
	collectedAt int64
	state       *pb.State
}

type StateSampleBuffer struct {
	mu      sync.Mutex
	samples map[string]liveStateSample
}

func NewStateSampleBuffer() *StateSampleBuffer {
	return &StateSampleBuffer{samples: map[string]liveStateSample{}}
}

func (b *StateSampleBuffer) Add(eventID, nodeUUID []byte, collectedAt int64, state *pb.State) {
	if b == nil || state == nil || len(eventID) == 0 || len(nodeUUID) != 16 {
		return
	}
	key := hex.EncodeToString(eventID)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.samples == nil {
		b.samples = map[string]liveStateSample{}
	}
	if _, exists := b.samples[key]; exists {
		return
	}
	b.samples[key] = liveStateSample{
		eventID: key, nodeUUID: append([]byte(nil), nodeUUID...), collectedAt: collectedAt,
		state: proto.Clone(state).(*pb.State),
	}
}

func (b *StateSampleBuffer) TakeBefore(cutoff int64) []liveStateSample {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.samples) == 0 {
		return nil
	}
	out := make([]liveStateSample, 0, len(b.samples))
	for key, sample := range b.samples {
		if sample.collectedAt < cutoff {
			out = append(out, sample)
			delete(b.samples, key)
		}
	}
	return out
}
