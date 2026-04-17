package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/Ivlyth/process-bandwidth/config"
	"github.com/Ivlyth/process-bandwidth/internal/collector"
	"github.com/Ivlyth/process-bandwidth/internal/store"
	"github.com/Ivlyth/process-bandwidth/ui/web"
)

// ──────────────────────────────────────────────────────────────
// cobra command
// ──────────────────────────────────────────────────────────────

func newTestCmd() *cobra.Command {
	var (
		webPort  uint16
		duration time.Duration
		seed     int64
	)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Integration self-test (requires root / CAP_BPF)",
		Long: `Runs an automated integration test by simulating known network and
file I/O traffic, then verifying that pbmon correctly measures it via eBPF.

Traffic is generated in two phases:
  Pre-BPF  – FDs opened before the BPF program loads (informational only)
  Post-BPF – FDs opened after BPF loads (full capture required, ±5% tolerance)

Web dashboard is available at http://localhost:<web-port>/ during the test.
Pass --web-port 0 to disable it.

Exits 0 on PASS, 1 on FAIL.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if seed == 0 {
				seed = time.Now().UnixNano()
			}
			return runIntegrationTest(webPort, duration, seed)
		},
	}

	cmd.Flags().Uint16Var(&webPort, "web-port", 18080, "Web server port (0 to disable)")
	cmd.Flags().DurationVar(&duration, "duration", 20*time.Second, "Total test duration")
	cmd.Flags().Int64Var(&seed, "seed", 0, "Random seed (0 = use current time)")

	return cmd
}

// ──────────────────────────────────────────────────────────────
// fdRecord: expected I/O for one open file descriptor
// ──────────────────────────────────────────────────────────────

type fdRecord struct {
	fd      int
	txBytes int64  // bytes written via write()/sendto() on this FD
	rxBytes int64  // bytes read via read()/recvfrom() on this FD
	preBPF  bool
	kind    string // "net" or "file"
}

// ──────────────────────────────────────────────────────────────
// Test orchestration
// ──────────────────────────────────────────────────────────────

func runIntegrationTest(webPort uint16, totalDur time.Duration, seed int64) error {
	banner("pbmon integration test", fmt.Sprintf("seed=%d  duration=%s", seed, totalDur))

	// Suppress collector log noise; errors only.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Test-specific config: loopback + file I/O both enabled, longer idle
	// timeout so nothing gets pruned during the test.
	cfg := config.Default()
	cfg.IncludeLocal = true
	cfg.IncludeFileIO = true
	cfg.WebPort = webPort
	cfg.IdleTimeout = 120 * time.Second

	// Echo server for TCP loopback simulation.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("echo listener: %w", err)
	}
	defer ln.Close()
	go runEchoServer(ln)
	echoAddr := ln.Addr().String()

	// Distribute seed so concurrent goroutines get independent RNGs.
	mkRng := func(delta int64) *rand.Rand { return rand.New(rand.NewSource(seed + delta)) }

	// ── Phase 1: Pre-BPF traffic ─────────────────────────────
	fmt.Println("\n[Phase 1] Pre-BPF traffic simulation (2 s)")
	preDur := 2 * time.Second

	var preNets, preFiles []*fdRecord
	var preConns []net.Conn
	var preFileHandles []*os.File

	{
		var wg sync.WaitGroup
		var mu sync.Mutex
		wg.Add(2)
		go func() {
			defer wg.Done()
			recs, conns := simNet(mkRng(10), echoAddr, 2, preDur)
			mu.Lock()
			preNets, preConns = recs, conns
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			recs, files := simFile(mkRng(20), 2, preDur)
			mu.Lock()
			preFiles, preFileHandles = recs, files
			mu.Unlock()
		}()
		wg.Wait()
	}

	defer closeConns(preConns)
	defer closeFiles(preFileHandles)

	fmt.Printf("         generated: %d net FDs, %d file FDs\n",
		len(preNets), len(preFiles))
	printFDSummary(preNets, preFiles)

	// Let TCP connections settle in /proc/net before BPF loads.
	time.Sleep(400 * time.Millisecond)

	// ── Phase 2: Load BPF ────────────────────────────────────
	fmt.Println("\n[Phase 2] Loading eBPF program…")
	coll, err := collector.New(&cfg, logger)
	if err != nil {
		return fmt.Errorf("collector init (need root / CAP_BPF): %w", err)
	}
	coll.Start()
	defer coll.Stop()
	fmt.Println("         ✓ BPF loaded, tracepoints attached")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if webPort > 0 {
		go func() { _ = web.Start(ctx, &cfg, coll.Store()) }()
		fmt.Printf("         web dashboard → http://localhost:%d/\n", webPort)
	}

	// ── Phase 3: Post-BPF traffic ────────────────────────────
	simDur := totalDur - preDur - 5*time.Second // reserve 5 s for settle + setup
	if simDur < 5*time.Second {
		simDur = 5 * time.Second
	}
	fmt.Printf("\n[Phase 3] Post-BPF traffic simulation (%.0f s)\n", simDur.Seconds())

	var postNets, postFiles []*fdRecord
	var postConns []net.Conn
	var postFileHandles []*os.File

	{
		var wg sync.WaitGroup
		var mu sync.Mutex
		wg.Add(2)
		go func() {
			defer wg.Done()
			recs, conns := simNet(mkRng(30), echoAddr, 3, simDur)
			mu.Lock()
			postNets, postConns = recs, conns
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			recs, files := simFile(mkRng(40), 3, simDur)
			mu.Lock()
			postFiles, postFileHandles = recs, files
			mu.Unlock()
		}()
		wg.Wait()
	}

	defer closeConns(postConns)
	defer closeFiles(postFileHandles)

	fmt.Printf("         generated: %d net FDs, %d file FDs\n",
		len(postNets), len(postFiles))
	printFDSummary(postNets, postFiles)

	// ── Phase 4: Wait for store to settle ────────────────────
	fmt.Println("\n[Phase 4] Waiting for measurements to settle (3 s)…")
	time.Sleep(3 * time.Second)

	// ── Phase 5: Verify ──────────────────────────────────────
	fmt.Println("\n[Phase 5] Verifying measurements")
	return verify(coll.Store(), uint32(os.Getpid()), postNets, postFiles, preNets, preFiles)
}

// ──────────────────────────────────────────────────────────────
// Echo server
// ──────────────────────────────────────────────────────────────

func runEchoServer(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 32*1024)
			for {
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				if _, err = c.Write(buf[:n]); err != nil {
					return
				}
			}
		}(conn)
	}
}

// ──────────────────────────────────────────────────────────────
// Network traffic simulation
// ──────────────────────────────────────────────────────────────

// simNet opens n TCP connections to the echo server and sends/receives data
// for dur. Returns the fdRecords (expected bytes) and the open net.Conns
// (caller must close them after verification).
func simNet(rng *rand.Rand, echoAddr string, n int, dur time.Duration) ([]*fdRecord, []net.Conn) {
	records := make([]*fdRecord, n)
	conns := make([]net.Conn, n)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		i := i
		// Each goroutine gets a unique RNG so they generate different data.
		workerRng := rand.New(rand.NewSource(rng.Int63()))
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", echoAddr)
			if err != nil {
				return
			}
			conns[i] = conn
			fd := extractConnFD(conn)
			rec := &fdRecord{fd: fd, kind: "net"}
			records[i] = rec

			chunkSize := 8*1024 + workerRng.Intn(24*1024) // 8–32 KB
			buf := make([]byte, chunkSize)
			workerRng.Read(buf)
			deadline := time.Now().Add(dur)

			for time.Now().Before(deadline) {
				n, err := conn.Write(buf)
				if err != nil {
					break
				}
				rec.txBytes += int64(n)

				// Read echo back (blocking until n bytes received).
				got := 0
				conn.SetReadDeadline(time.Now().Add(10 * time.Second))
				for got < n {
					m, err := conn.Read(buf[got:n])
					got += m
					if err != nil {
						goto done
					}
				}
				conn.SetReadDeadline(time.Time{})
				rec.rxBytes += int64(got)

				// Brief pause: keeps event rate manageable.
				time.Sleep(time.Duration(2+workerRng.Intn(8)) * time.Millisecond)
			}
		done:
		}()
	}
	wg.Wait()

	// Filter out goroutines that failed to connect.
	var outRecs []*fdRecord
	var outConns []net.Conn
	for i := range records {
		if records[i] != nil && records[i].fd > 2 {
			outRecs = append(outRecs, records[i])
			outConns = append(outConns, conns[i])
		}
	}
	return outRecs, outConns
}

// extractConnFD extracts the raw OS file descriptor from a net.Conn.
// Returns -1 on failure. Does not affect the connection's behaviour.
func extractConnFD(conn net.Conn) int {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		return -1
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		return -1
	}
	fd := -1
	_ = raw.Control(func(f uintptr) { fd = int(f) })
	return fd
}

// ──────────────────────────────────────────────────────────────
// File traffic simulation
// ──────────────────────────────────────────────────────────────

// simFile opens n temp files and writes/reads data for dur.
// The file is written for dur, then read back in full.
// Returns fdRecords and the open *os.File handles (caller must close).
func simFile(rng *rand.Rand, n int, dur time.Duration) ([]*fdRecord, []*os.File) {
	records := make([]*fdRecord, n)
	files := make([]*os.File, n)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		i := i
		workerRng := rand.New(rand.NewSource(rng.Int63()))
		go func() {
			defer wg.Done()
			f, err := os.CreateTemp("", "pbmon-itest-*")
			if err != nil {
				return
			}
			// Unlink path immediately; file lives until the handle is closed.
			os.Remove(f.Name())

			// Calling f.Fd() puts the file in blocking mode but we only do
			// sequential access from this goroutine, so that's fine.
			fd := int(f.Fd())
			rec := &fdRecord{fd: fd, kind: "file"}
			records[i] = rec
			files[i] = f

			chunkSize := 32*1024 + workerRng.Intn(96*1024) // 32–128 KB
			buf := make([]byte, chunkSize)
			workerRng.Read(buf)
			deadline := time.Now().Add(dur)

			for time.Now().Before(deadline) {
				n, err := f.Write(buf)
				if err != nil {
					break
				}
				rec.txBytes += int64(n)
				time.Sleep(time.Duration(3+workerRng.Intn(7)) * time.Millisecond)
			}
			f.Sync()

			// Seek to start and read back everything we wrote.
			if _, err := f.Seek(0, io.SeekStart); err == nil {
				readBuf := make([]byte, chunkSize)
				for {
					m, err := f.Read(readBuf)
					rec.rxBytes += int64(m)
					if err != nil {
						break
					}
				}
			}
		}()
	}
	wg.Wait()

	var outRecs []*fdRecord
	var outFiles []*os.File
	for i := range records {
		if records[i] != nil && records[i].fd > 2 {
			outRecs = append(outRecs, records[i])
			outFiles = append(outFiles, files[i])
		}
	}
	return outRecs, outFiles
}

// ──────────────────────────────────────────────────────────────
// Cleanup helpers
// ──────────────────────────────────────────────────────────────

func closeConns(conns []net.Conn) {
	for _, c := range conns {
		if c != nil {
			c.Close()
		}
	}
}

func closeFiles(files []*os.File) {
	for _, f := range files {
		if f != nil {
			f.Close()
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Verification
// ──────────────────────────────────────────────────────────────

const testTolerance = 0.05 // 5 %

type checkLine struct {
	label    string
	expected int64
	actual   uint64
	pass     bool
}

func verify(s *store.Store, pid uint32, postNets, postFiles, preNets, preFiles []*fdRecord) error {
	proc := s.Get(pid)
	if proc == nil {
		return fmt.Errorf("FAIL: PID %d not found in store — no I/O events were captured", pid)
	}

	var lines []checkLine
	allPass := true

	check := func(label string, expectedBytes int64, fd int, wantTx bool) {
		conn := proc.GetConnection(uint32(fd))
		var actual uint64
		if conn != nil {
			if wantTx {
				actual = conn.IO.TotalTx()
			} else {
				actual = conn.IO.TotalRx()
			}
		}
		pass := conn != nil && actual >= uint64(float64(expectedBytes)*(1-testTolerance))
		if !pass {
			allPass = false
		}
		lines = append(lines, checkLine{label, expectedBytes, actual, pass})
	}

	fmt.Printf("\n  Post-BPF network FDs — must pass (±%.0f%%):\n", testTolerance*100)
	for _, r := range postNets {
		if r.fd <= 2 {
			continue
		}
		check(fmt.Sprintf("    net fd=%-4d TX", r.fd), r.txBytes, r.fd, true)
		check(fmt.Sprintf("    net fd=%-4d RX", r.fd), r.rxBytes, r.fd, false)
	}

	fmt.Printf("\n  Post-BPF file FDs — must pass (±%.0f%%):\n", testTolerance*100)
	for _, r := range postFiles {
		if r.fd <= 2 {
			continue
		}
		check(fmt.Sprintf("    file fd=%-3d write", r.fd), r.txBytes, r.fd, true)
		check(fmt.Sprintf("    file fd=%-3d read ", r.fd), r.rxBytes, r.fd, false)
	}

	// Print table
	fmt.Printf("\n  %-30s %14s %14s  %s\n", "Check", "Expected", "Actual", "Result")
	fmt.Printf("  %s\n", strings.Repeat("─", 70))
	for _, l := range lines {
		status := "✓ PASS"
		if !l.pass {
			status = "✗ FAIL"
		}
		fmt.Printf("  %-30s %14s %14s  %s\n",
			l.label, fmtB(uint64(l.expected)), fmtB(l.actual), status)
	}

	// Pre-BPF: informational summary only (partial capture expected)
	fmt.Printf("\n  Pre-BPF FDs — informational (partial capture expected):\n")
	for _, r := range append(preNets, preFiles...) {
		if r.fd <= 2 {
			continue
		}
		conn := proc.GetConnection(uint32(r.fd))
		var capTx, capRx uint64
		pctTx, pctRx := "  n/a", "  n/a"
		if conn != nil {
			capTx = conn.IO.TotalTx()
			capRx = conn.IO.TotalRx()
			if r.txBytes > 0 {
				pctTx = fmt.Sprintf("%3.0f%%", float64(capTx)/float64(r.txBytes)*100)
			}
			if r.rxBytes > 0 {
				pctRx = fmt.Sprintf("%3.0f%%", float64(capRx)/float64(r.rxBytes)*100)
			}
		}
		fmt.Printf("    %s fd=%-4d  expected TX %-10s captured TX %-10s (%s)  expected RX %-10s captured RX %-10s (%s)\n",
			r.kind, r.fd,
			fmtB(uint64(r.txBytes)), fmtB(capTx), pctTx,
			fmtB(uint64(r.rxBytes)), fmtB(capRx), pctRx)
	}

	fmt.Println()
	if allPass {
		banner("RESULT: PASS ✓", "")
		return nil
	}
	banner("RESULT: FAIL ✗", "one or more post-BPF FD checks did not meet the 5% tolerance")
	return fmt.Errorf("integration test FAILED")
}

// ──────────────────────────────────────────────────────────────
// Formatting helpers
// ──────────────────────────────────────────────────────────────

func fmtB(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func printFDSummary(nets, files []*fdRecord) {
	for _, r := range nets {
		fmt.Printf("         net  fd=%-4d  TX %-12s  RX %s\n",
			r.fd, fmtB(uint64(r.txBytes)), fmtB(uint64(r.rxBytes)))
	}
	for _, r := range files {
		fmt.Printf("         file fd=%-4d  write %-12s  read %s\n",
			r.fd, fmtB(uint64(r.txBytes)), fmtB(uint64(r.rxBytes)))
	}
}

func banner(title, sub string) {
	fmt.Println(strings.Repeat("═", 56))
	fmt.Printf("  %s\n", title)
	if sub != "" {
		fmt.Printf("  %s\n", sub)
	}
	fmt.Println(strings.Repeat("═", 56))
}
