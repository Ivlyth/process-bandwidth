package model

import (
	"sync"
	"sync/atomic"
	"time"
)

// Process represents a monitored process.
// Net and File are separate IOCounters: Net tracks network socket I/O,
// File tracks regular file and pipe I/O.
type Process struct {
	PID uint32

	mu      sync.RWMutex
	name    string
	cmdline string

	Net  *IOCounter // network I/O (FDClassSocket only)
	File *IOCounter // file + pipe I/O (FDClassFile + FDClassPipe)

	// Connections is a per-FD map. Keyed by uint32 (fd).
	// Uses sync.Map for lock-free concurrent access from worker goroutines.
	Connections sync.Map

	createdAt    time.Time
	lastActivity atomic.Int64 // unix nano
}

// NewProcess creates a Process with the given PID and history length.
func NewProcess(pid uint32, histLen int) *Process {
	p := &Process{
		PID:       pid,
		Net:       NewIOCounter(histLen),
		File:      NewIOCounter(histLen),
		createdAt: time.Now(),
	}
	p.lastActivity.Store(time.Now().UnixNano())
	return p
}

// SetMeta updates the process name and command line.
func (p *Process) SetMeta(name, cmdline string) {
	p.mu.Lock()
	p.name = name
	p.cmdline = cmdline
	p.mu.Unlock()
}

// Name returns the process name (e.g. "nginx").
func (p *Process) Name() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.name
}

// Cmdline returns the full command line.
func (p *Process) Cmdline() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cmdline
}

// Touch updates the last-activity timestamp.
func (p *Process) Touch() {
	p.lastActivity.Store(time.Now().UnixNano())
}

// LastActivity returns when the process last had I/O activity.
func (p *Process) LastActivity() time.Time {
	return time.Unix(0, p.lastActivity.Load())
}

// CreatedAt returns when the process was first seen.
func (p *Process) CreatedAt() time.Time { return p.createdAt }

// IsIdle returns true if the process has had no activity for the given duration.
func (p *Process) IsIdle(timeout time.Duration) bool {
	return time.Since(p.LastActivity()) > timeout
}

// GetOrCreateConnection returns the existing Connection for fd, or creates a new one.
func (p *Process) GetOrCreateConnection(fd uint32, class FDClass, histLen int) (*Connection, bool) {
	actual, loaded := p.Connections.LoadOrStore(fd, NewConnection(fd, class, histLen))
	return actual.(*Connection), loaded
}

// GetConnection returns the Connection for fd, or nil if not found.
func (p *Process) GetConnection(fd uint32) *Connection {
	v, ok := p.Connections.Load(fd)
	if !ok {
		return nil
	}
	return v.(*Connection)
}

// DeleteConnection removes the Connection for fd.
func (p *Process) DeleteConnection(fd uint32) {
	p.Connections.Delete(fd)
}

// EachConnection iterates over all connections, calling fn for each.
// Iteration continues until fn returns false or all connections are visited.
func (p *Process) EachConnection(fn func(*Connection) bool) {
	p.Connections.Range(func(_, v any) bool {
		return fn(v.(*Connection))
	})
}

// ConnectionCount returns the number of tracked connections.
func (p *Process) ConnectionCount() int {
	n := 0
	p.Connections.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}
