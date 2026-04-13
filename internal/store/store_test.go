package store

import (
	"sync"
	"testing"
)

func TestStoreGetOrCreate(t *testing.T) {
	s := New(60)

	p1 := s.GetOrCreate(100)
	if p1 == nil {
		t.Fatal("GetOrCreate returned nil")
	}
	if p1.PID != 100 {
		t.Errorf("PID: want 100, got %d", p1.PID)
	}

	// Same PID should return same pointer
	p2 := s.GetOrCreate(100)
	if p1 != p2 {
		t.Error("GetOrCreate for same PID returned different pointers")
	}

	// Different PID should return different pointer
	p3 := s.GetOrCreate(200)
	if p3 == p1 {
		t.Error("GetOrCreate for different PID returned same pointer")
	}
}

func TestStoreDelete(t *testing.T) {
	s := New(60)

	s.GetOrCreate(42)
	if s.ProcessCount() != 1 {
		t.Fatalf("want 1 process, got %d", s.ProcessCount())
	}

	s.Delete(42)
	if s.ProcessCount() != 0 {
		t.Fatalf("want 0 processes after delete, got %d", s.ProcessCount())
	}

	if p := s.Get(42); p != nil {
		t.Error("Get after Delete should return nil")
	}
}

func TestStoreProcesses(t *testing.T) {
	s := New(60)
	for i := uint32(1); i <= 5; i++ {
		s.GetOrCreate(i)
	}
	procs := s.Processes()
	if len(procs) != 5 {
		t.Errorf("want 5 processes, got %d", len(procs))
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	s := New(60)
	const goroutines = 50
	const pids = 20

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pid := uint32(i % pids)
			p := s.GetOrCreate(pid)
			if p == nil {
				t.Errorf("GetOrCreate returned nil for pid %d", pid)
			}
		}(i)
	}
	wg.Wait()

	// After all goroutines, we should have exactly pids processes
	if s.ProcessCount() != pids {
		t.Errorf("want %d processes, got %d", pids, s.ProcessCount())
	}
}

func TestStoreDropped(t *testing.T) {
	s := New(60)
	s.AddDropped(5)
	s.AddDropped(3)
	if s.Dropped() != 8 {
		t.Errorf("Dropped: want 8, got %d", s.Dropped())
	}
}
