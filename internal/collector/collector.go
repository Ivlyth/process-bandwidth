// Package collector is the core data collection engine for pbmon.
// It loads the eBPF program, reads events, and updates the Store.
package collector

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Ivlyth/process-bandwidth/config"
	bpfpkg "github.com/Ivlyth/process-bandwidth/internal/bpf"
	"github.com/Ivlyth/process-bandwidth/internal/model"
	"github.com/Ivlyth/process-bandwidth/internal/store"
)

// Collector is the main collection engine.
// Create with New(), call Start() to begin collection, Stop() to clean up.
type Collector struct {
	cfg    *config.Config
	logger *slog.Logger

	objs   *bpfpkg.Objects
	reader bpfpkg.EventReader
	store  *store.Store

	netRes  *NetResolver
	procRes *ProcResolver

	rawCh  chan []byte
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// dropped event count (perf ring buffer overflows)
	dropped atomic.Uint64
}

// New creates a Collector. It loads the eBPF program but does not start
// reading events yet. Call Start() to begin.
func New(cfg *config.Config, logger *slog.Logger) (*Collector, error) {
	objs, err := bpfpkg.Load()
	if err != nil {
		return nil, err
	}

	reader, err := bpfpkg.NewReader(objs)
	if err != nil {
		objs.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Collector{
		cfg:     cfg,
		logger:  logger,
		objs:    objs,
		reader:  reader,
		store:   store.New(cfg.SnapshotCount),
		netRes:  NewNetResolver(cfg.NetRefreshInterval),
		procRes: &ProcResolver{},
		rawCh:   make(chan []byte, cfg.ChannelSize),
		ctx:     ctx,
		cancel:  cancel,
	}
	return c, nil
}

// Store returns the data store for reading by UI / API layers.
func (c *Collector) Store() *store.Store { return c.store }

// Start begins event collection.
func (c *Collector) Start() {
	c.netRes.Start(c.ctx)

	c.wg.Add(1)
	go c.readLoop()

	workers := c.cfg.WorkersCount
	if workers <= 0 {
		workers = 2
	}
	for i := 0; i < workers; i++ {
		c.wg.Add(1)
		go c.workerLoop()
	}

	c.wg.Add(1)
	go c.snapshotLoop()
}

// Stop gracefully shuts down all goroutines and releases eBPF resources.
func (c *Collector) Stop() {
	c.cancel()          // signal all goroutines
	c.reader.Close()    // unblock readLoop's blocked Read()
	c.wg.Wait()         // wait for readLoop + workers + snapshotLoop
	c.objs.Close()      // detach tracepoints + free kernel maps/progs
}

// readLoop reads raw events from the BPF ring/perf reader and forwards
// them to rawCh for processing by worker goroutines.
func (c *Collector) readLoop() {
	defer func() {
		close(c.rawCh) // signal workers that no more events are coming
		c.wg.Done()
	}()

	for {
		raw, err := c.reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return // reader closed, normal shutdown
			}
			var dropped *bpfpkg.DroppedSamplesError
			if errors.As(err, &dropped) {
				c.store.AddDropped(dropped.Count)
				continue
			}
			// Other errors: log and continue unless context is done.
			select {
			case <-c.ctx.Done():
				return
			default:
				c.logger.Warn("event reader error", "err", err)
				continue
			}
		}

		// Non-blocking send: if the channel is full, drop and count.
		select {
		case c.rawCh <- raw:
		default:
			c.store.AddDropped(1)
		}
	}
}

// workerLoop decodes and processes events from rawCh.
func (c *Collector) workerLoop() {
	defer c.wg.Done()
	for raw := range c.rawCh {
		ev, err := DecodeEvent(raw)
		if err != nil {
			continue
		}
		switch e := ev.(type) {
		case *IOEvent:
			c.handleIO(e)
		case *FDEvent:
			c.handleFD(e)
		case *ProcEvent:
			c.handleProc(e)
		}
	}
}

// handleIO processes a read/write syscall event.
func (c *Collector) handleIO(e *IOEvent) {
	// Optional PID filter
	if len(c.cfg.FilterPIDs) > 0 && !containsUint32(c.cfg.FilterPIDs, e.PID) {
		return
	}

	proc := c.store.GetOrCreate(e.PID)
	c.procRes.EnsureResolved(e.PID, proc)
	proc.Touch()

	// Optional name filter (applied after we have the name)
	if len(c.cfg.FilterNames) > 0 {
		name := proc.Name()
		if name == "" || !matchesAny(name, c.cfg.FilterNames) {
			return
		}
	}

	conn, _ := proc.GetOrCreateConnection(e.FD, model.FDClass(e.FDClass), c.cfg.SnapshotCount)
	conn.Touch()

	// Update per-connection counters
	if e.Direction == dirWrite {
		conn.IO.AddTx(e.Bytes)
	} else {
		conn.IO.AddRx(e.Bytes)
	}

	// Lazily resolve socket connection info
	if model.FDClass(e.FDClass) == model.FDClassSocket && conn.Info() == nil {
		if info := c.netRes.LookupByFD(e.PID, e.FD); info != nil {
			conn.SetInfo(info)
		}
	}

	// Update process-level aggregate counters.
	// Network and File I/O are tracked separately.
	switch model.FDClass(e.FDClass) {
	case model.FDClassSocket:
		if !c.cfg.IncludeLocal {
			// Skip loopback connections
			if info := conn.Info(); info != nil && isLoopback(info.Remote) {
				return
			}
		}
		if e.Direction == dirWrite {
			proc.Net.AddTx(e.Bytes)
		} else {
			proc.Net.AddRx(e.Bytes)
		}

	case model.FDClassFile, model.FDClassPipe:
		if !c.cfg.IncludeFileIO {
			return
		}
		if e.Direction == dirWrite {
			proc.File.AddTx(e.Bytes)
		} else {
			proc.File.AddRx(e.Bytes)
		}
	}
}

// handleFD processes an fd lifecycle event (open/close).
func (c *Collector) handleFD(e *FDEvent) {
	if e.Op == fdOpClose {
		proc := c.store.Get(e.PID)
		if proc != nil {
			proc.DeleteConnection(e.FD)
		}
	}
	// FD_OP_OPEN: connection will be created on first IO event.
}

// handleProc processes a process lifecycle event (exit/fork).
func (c *Collector) handleProc(e *ProcEvent) {
	if e.Op == procOpExit {
		c.store.Delete(e.PID)
	}
	// Fork: new process will be created on first IO event.
}

// snapshotLoop takes per-second snapshots of all IOCounters and purges idle
// processes/connections.
func (c *Collector) snapshotLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case now := <-ticker.C:
			c.takeSnapshot(now)
		}
	}
}

func (c *Collector) takeSnapshot(now time.Time) {
	idleTimeout := c.cfg.IdleTimeout
	histLen := c.cfg.SnapshotCount

	c.store.WithWriteLock(func(procs map[uint32]*model.Process) {
		for pid, proc := range procs {
			if proc.IsIdle(idleTimeout) {
				delete(procs, pid)
				continue
			}
			proc.Net.TakeSample(now)
			proc.File.TakeSample(now)

			proc.EachConnection(func(conn *model.Connection) bool {
				if conn.IsIdle(idleTimeout) {
					proc.DeleteConnection(conn.FD)
					return true
				}
				conn.IO.TakeSample(now)
				_ = histLen
				return true
			})
		}
	})
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

func containsUint32(slice []uint32, v uint32) bool {
	for _, x := range slice {
		if x == v {
			return true
		}
	}
	return false
}

func matchesAny(name string, patterns []string) bool {
	nameLower := toLower(name)
	for _, p := range patterns {
		if contains(nameLower, toLower(p)) {
			return true
		}
	}
	return false
}

func isLoopback(addr string) bool {
	if addr == "" {
		return false
	}
	// Check for 127.x.x.x or [::1]
	return len(addr) >= 7 && (addr[:4] == "127." || addr[:5] == "[::1]")
}

// toLower is a simple ASCII lowercase to avoid importing strings in hot path.
func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
