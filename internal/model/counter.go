// Package model defines the core data structures for pbmon.
package model

import (
	"sync"
	"sync/atomic"
	"time"
)

// IOSample is a single 1-second measurement of I/O activity.
type IOSample struct {
	At       time.Time
	RxBytes  uint64
	TxBytes  uint64
	Duration time.Duration // time since previous sample (usually ~1 s)
}

// RxRateBps returns the receive rate in bytes/second for this sample.
func (s IOSample) RxRateBps() float64 {
	if s.Duration <= 0 {
		return 0
	}
	return float64(s.RxBytes) / s.Duration.Seconds()
}

// TxRateBps returns the transmit rate in bytes/second for this sample.
func (s IOSample) TxRateBps() float64 {
	if s.Duration <= 0 {
		return 0
	}
	return float64(s.TxBytes) / s.Duration.Seconds()
}

// IOCounter tracks cumulative I/O bytes and a fixed-capacity ring of recent
// per-second samples.
//
// Bytes are updated by worker goroutines via AddRx/AddTx using atomic operations
// (no lock required). The history ring is updated once per second by the snapshot
// worker and protected by mu.
type IOCounter struct {
	totalRx atomic.Uint64 // cumulative received / read bytes
	totalTx atomic.Uint64 // cumulative sent / written bytes

	mu      sync.Mutex
	history []IOSample // circular buffer, capacity = histCap
	histCap int
	head    int // next write position

	lastRx  uint64
	lastTx  uint64
	lastAt  time.Time
}

// NewIOCounter creates an IOCounter with the given history capacity.
func NewIOCounter(historyLen int) *IOCounter {
	if historyLen <= 0 {
		historyLen = 60
	}
	return &IOCounter{
		histCap: historyLen,
		history: make([]IOSample, 0, historyLen),
	}
}

// AddRx adds n received/read bytes.
func (c *IOCounter) AddRx(n uint64) { c.totalRx.Add(n) }

// AddTx adds n sent/written bytes.
func (c *IOCounter) AddTx(n uint64) { c.totalTx.Add(n) }

// TotalRx returns the cumulative received bytes.
func (c *IOCounter) TotalRx() uint64 { return c.totalRx.Load() }

// TotalTx returns the cumulative sent bytes.
func (c *IOCounter) TotalTx() uint64 { return c.totalTx.Load() }

// TakeSample is called by the snapshot worker once per second.
// It computes the delta since the last call and appends it to the history ring.
func (c *IOCounter) TakeSample(now time.Time) IOSample {
	rx := c.totalRx.Load()
	tx := c.totalTx.Load()

	var dur time.Duration
	if !c.lastAt.IsZero() {
		dur = now.Sub(c.lastAt)
	} else {
		dur = time.Second
	}

	sample := IOSample{
		At:       now,
		RxBytes:  rx - c.lastRx,
		TxBytes:  tx - c.lastTx,
		Duration: dur,
	}

	c.lastRx = rx
	c.lastTx = tx
	c.lastAt = now

	c.mu.Lock()
	if len(c.history) < c.histCap {
		c.history = append(c.history, sample)
	} else {
		c.history[c.head] = sample
		c.head = (c.head + 1) % c.histCap
	}
	c.mu.Unlock()

	return sample
}

// LastSample returns the most recent sample, or a zero sample if none exist.
func (c *IOCounter) LastSample() IOSample {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.history) == 0 {
		return IOSample{}
	}
	idx := (c.head - 1 + c.histCap) % c.histCap
	if idx >= len(c.history) {
		return c.history[len(c.history)-1]
	}
	return c.history[idx]
}

// History returns a copy of recent samples in chronological order.
func (c *IOCounter) History() []IOSample {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.history) == 0 {
		return nil
	}
	out := make([]IOSample, len(c.history))
	// If the ring hasn't wrapped, just copy straight.
	if len(c.history) < c.histCap {
		copy(out, c.history)
		return out
	}
	// Ring has wrapped: start at head (oldest) and go round.
	n := copy(out, c.history[c.head:])
	copy(out[n:], c.history[:c.head])
	return out
}

// RxRate returns the current receive rate in bytes/second (from last sample).
func (c *IOCounter) RxRate() float64 { return c.LastSample().RxRateBps() }

// TxRate returns the current transmit rate in bytes/second (from last sample).
func (c *IOCounter) TxRate() float64 { return c.LastSample().TxRateBps() }
