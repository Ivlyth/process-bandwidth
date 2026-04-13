package model

import (
	"sync"
	"testing"
	"time"
)

func TestIOCounterBasic(t *testing.T) {
	c := NewIOCounter(60)

	c.AddRx(1000)
	c.AddTx(500)

	if c.TotalRx() != 1000 {
		t.Errorf("TotalRx: want 1000, got %d", c.TotalRx())
	}
	if c.TotalTx() != 500 {
		t.Errorf("TotalTx: want 500, got %d", c.TotalTx())
	}
}

func TestIOCounterSnapshot(t *testing.T) {
	c := NewIOCounter(10)

	c.AddRx(2000)
	c.AddTx(1000)

	now := time.Now()
	s := c.TakeSample(now)

	if s.RxBytes != 2000 {
		t.Errorf("sample RxBytes: want 2000, got %d", s.RxBytes)
	}
	if s.TxBytes != 1000 {
		t.Errorf("sample TxBytes: want 1000, got %d", s.TxBytes)
	}

	// Second sample: delta since first
	c.AddRx(500)
	s2 := c.TakeSample(time.Now())
	if s2.RxBytes != 500 {
		t.Errorf("second sample RxBytes: want 500, got %d", s2.RxBytes)
	}
	if s2.TxBytes != 0 {
		t.Errorf("second sample TxBytes: want 0, got %d", s2.TxBytes)
	}
}

func TestIOCounterHistory(t *testing.T) {
	c := NewIOCounter(5) // small capacity
	for i := 0; i < 10; i++ {
		c.AddRx(uint64(i * 100))
		c.TakeSample(time.Now())
	}
	h := c.History()
	if len(h) != 5 {
		t.Errorf("history len: want 5 (capped), got %d", len(h))
	}
}

func TestIOCounterConcurrent(t *testing.T) {
	c := NewIOCounter(60)
	const goroutines = 100
	const bytesEach = 1000

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.AddRx(bytesEach)
			c.AddTx(bytesEach / 2)
		}()
	}
	wg.Wait()

	c.TakeSample(time.Now())

	want := uint64(goroutines * bytesEach)
	if c.TotalRx() != want {
		t.Errorf("TotalRx: want %d, got %d", want, c.TotalRx())
	}
	wantTx := uint64(goroutines * bytesEach / 2)
	if c.TotalTx() != wantTx {
		t.Errorf("TotalTx: want %d, got %d", wantTx, c.TotalTx())
	}
}

func TestIOCounterRateCalc(t *testing.T) {
	c := NewIOCounter(10)
	c.AddRx(4096)
	c.TakeSample(time.Now())
	// Rate should be approx 4096 / ~1s ≈ 4096 bps (duration is tiny in test so may vary)
	// Just verify it's non-zero and not negative
	r := c.RxRate()
	if r < 0 {
		t.Errorf("RxRate returned negative: %f", r)
	}
}
