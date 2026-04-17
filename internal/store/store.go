// Package store provides the thread-safe in-memory data store for pbmon.
// All monitored processes and connections are kept here.
//
// Design notes:
//   - store.mu (sync.RWMutex) protects the procs map.
//     Writers: GetOrCreate (Lock), Delete (Lock), snapshot worker (Lock)
//     Readers: Processes/Get (RLock), TUI/API goroutines (RLock)
//   - Per-process Connection maps use sync.Map for lock-free worker access.
//   - IOCounter byte fields use atomic.Uint64 – no lock needed for Add/Load.
//   - IOCounter history is protected by IOCounter.mu (separate from store.mu).
package store

import (
	"sync"
	"sync/atomic"

	"github.com/Ivlyth/process-bandwidth/internal/model"
)

// Store is the central in-memory store.
type Store struct {
	mu            sync.RWMutex
	procs         map[uint32]*model.Process
	droppedEvents atomic.Uint64
	histLen       int
}

// New creates an empty Store with the given history length for IOCounters.
func New(histLen int) *Store {
	return &Store{
		procs:   make(map[uint32]*model.Process),
		histLen: histLen,
	}
}

// GetOrCreate returns the Process for pid, creating it if it doesn't exist.
func (s *Store) GetOrCreate(pid uint32) *model.Process {
	// Fast path: read lock
	s.mu.RLock()
	p, ok := s.procs[pid]
	s.mu.RUnlock()
	if ok {
		return p
	}

	// Slow path: write lock
	s.mu.Lock()
	// Double-check after acquiring write lock
	if p, ok = s.procs[pid]; ok {
		s.mu.Unlock()
		return p
	}
	p = model.NewProcess(pid, s.histLen)
	s.procs[pid] = p
	s.mu.Unlock()
	return p
}

// Get returns the Process for pid, or nil if not found.
func (s *Store) Get(pid uint32) *model.Process {
	s.mu.RLock()
	p := s.procs[pid]
	s.mu.RUnlock()
	return p
}

// Delete removes the process with the given pid.
func (s *Store) Delete(pid uint32) {
	s.mu.Lock()
	delete(s.procs, pid)
	s.mu.Unlock()
}

// Processes returns a snapshot slice of all tracked processes.
// Callers must not modify the returned slice or the Process values.
func (s *Store) Processes() []*model.Process {
	s.mu.RLock()
	out := make([]*model.Process, 0, len(s.procs))
	for _, p := range s.procs {
		out = append(out, p)
	}
	s.mu.RUnlock()
	return out
}

// ProcessCount returns the number of tracked processes.
func (s *Store) ProcessCount() int {
	s.mu.RLock()
	n := len(s.procs)
	s.mu.RUnlock()
	return n
}

// AddDropped increments the dropped-event counter by n.
func (s *Store) AddDropped(n uint64) {
	s.droppedEvents.Add(n)
}

// Dropped returns the total number of dropped events since startup.
func (s *Store) Dropped() uint64 {
	return s.droppedEvents.Load()
}

// WithWriteLock executes fn while holding the store write lock.
// Used by the snapshot worker to atomically iterate + purge processes.
func (s *Store) WithWriteLock(fn func(procs map[uint32]*model.Process)) {
	s.mu.Lock()
	fn(s.procs)
	s.mu.Unlock()
}
