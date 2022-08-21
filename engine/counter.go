package engine

import (
	"github.com/Ivlyth/process-bandwidth/config"
	"github.com/Ivlyth/process-bandwidth/pkg/ring"
	"go.uber.org/atomic"
	"sync"
	"time"
)

type RWEventCounter struct {
	CreatedAt    time.Time
	LastUpdateAt time.Time

	outgoingBytes uint64
	incomingBytes uint64

	OutgoingBytes atomic.Uint64
	IncomingBytes atomic.Uint64

	lastOutgoingBytes uint64
	lastIncomingBytes uint64
	lastSnapshotAt    time.Time // 最近一次计算带宽的时间

	ring ring.Ringer[*RWSnapshot]

	once sync.Once // 用于首次初始化
}

func (rw *RWEventCounter) isIdle() bool {
	return !rw.LastUpdateAt.IsZero() && time.Now().Sub(rw.LastUpdateAt) > config.GlobalConfig.IdleTimeout
}

func (rw *RWEventCounter) addByRWEvent(rwEvent *RWEvent) {
	rw.LastUpdateAt = time.Now()
	switch rwEvent.Direction {
	case DirectionIncoming:
		rw.IncomingBytes.Add(rwEvent.Size)
	case DirectionOutgoing:
		rw.OutgoingBytes.Add(rwEvent.Size)
	}
}

func (rw *RWEventCounter) takeSnapshot() *RWSnapshot {
	if rw.ring == nil {
		return nil
	}

	now := time.Now()

	rw.outgoingBytes = rw.OutgoingBytes.Load()
	rw.incomingBytes = rw.IncomingBytes.Load()

	outgoingBytes := rw.outgoingBytes - rw.lastOutgoingBytes
	incomingBytes := rw.incomingBytes - rw.lastIncomingBytes

	var s *RWSnapshot

	if !rw.lastSnapshotAt.IsZero() {
		duration := now.Sub(rw.lastSnapshotAt)

		rw.ring = rw.ring.Next()
		s = rw.ring.GetValue()
		if s != nil {
			s.OutgoingBytes = outgoingBytes
			s.IncomingBytes = incomingBytes
			s.At = now
			s.Duration = duration
		} else {
			s = &RWSnapshot{
				OutgoingBytes: outgoingBytes,
				IncomingBytes: incomingBytes,
				At:            now,
				Duration:      duration,
			}
			rw.ring.SetValue(s)
		}
	}

	rw.lastOutgoingBytes = rw.outgoingBytes
	rw.lastIncomingBytes = rw.incomingBytes
	rw.lastSnapshotAt = now

	return s
}

func (rw *RWEventCounter) Init(f func()) {
	rw.once.Do(func() {
		rw.CreatedAt = time.Now()
		rw.LastUpdateAt = time.Now()
		//rw.ring = RingPool.Get()
		rw.ring = ring.NewArrayRing[*RWSnapshot](config.GlobalConfig.SnapShotCount)
		//rw.ring = ring.NewLinkRing[*RWSnapshot](config.GlobalConfig.SnapShotCount)
		f()
	})
}

func (rw *RWEventCounter) LastSnapshot() RWSnapshot {
	if rw.ring == nil {
		return RWSnapshot{}
	}
	s := rw.ring.GetValue()
	if s != nil {
		return *s
	}
	return RWSnapshot{}
}

func (rw *RWEventCounter) Histories() []RWSnapshot {
	if rw.ring == nil {
		return []RWSnapshot{}
	}
	results := make([]RWSnapshot, 0, rw.ring.Len())
	rw.ring.Next().Do(func(s *RWSnapshot) {
		results = append(results, s.Clone())
	})
	return results
}
