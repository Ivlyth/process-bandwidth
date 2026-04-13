// pbmon – process bandwidth monitor
// eBPF-based per-process network and file I/O bandwidth monitoring.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Ivlyth/process-bandwidth/config"
	"github.com/Ivlyth/process-bandwidth/internal/collector"
	"github.com/Ivlyth/process-bandwidth/ui/tui"
	"github.com/Ivlyth/process-bandwidth/ui/web"
	"github.com/Ivlyth/process-bandwidth/version"
)

func main() {
	if err := newRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRoot() *cobra.Command {
	cfg := config.Default()
	var logPath string
	var noTUI bool

	root := &cobra.Command{
		Use:   "pbmon",
		Short: "eBPF-based per-process network and file I/O bandwidth monitor",
		Long: `pbmon monitors per-process network bandwidth (RX/TX) and optionally
file I/O bandwidth (reads/writes) using eBPF syscall tracepoints.

Requires root or CAP_BPF + CAP_PERFMON capabilities.
Minimum Linux kernel version: 4.9`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(&cfg, logPath, noTUI)
		},
	}

	f := root.Flags()
	f.BoolVar(&cfg.Debug, "debug", false, "Enable verbose debug logging")
	f.StringVar(&logPath, "log", "", "Log output path (default: stderr)")
	f.IntVar(&cfg.WorkersCount, "workers", cfg.WorkersCount, "Number of event-processing worker goroutines")
	f.IntVar(&cfg.ChannelSize, "channel-size", cfg.ChannelSize, "Event channel buffer size")
	f.IntVar(&cfg.SnapshotCount, "history", cfg.SnapshotCount, "Number of 1-second history samples to keep")
	f.DurationVar(&cfg.IdleTimeout, "idle-timeout", cfg.IdleTimeout, "Remove processes/connections idle for this duration")
	f.BoolVar(&cfg.IncludeFileIO, "file-io", false, "Track file and pipe I/O in addition to network I/O")
	f.BoolVar(&cfg.IncludeLocal, "local", false, "Include loopback (127.x / ::1) network traffic")
	f.Uint16Var(&cfg.WebPort, "web-port", 0, "HTTP server port (0 = disabled)")
	f.StringVar(&cfg.MetricsPath, "metrics-path", cfg.MetricsPath, "Prometheus metrics endpoint path")
	f.DurationVar(&cfg.NetRefreshInterval, "net-refresh", cfg.NetRefreshInterval, "Interval for refreshing /proc/net/* cache")
	f.BoolVar(&noTUI, "no-tui", false, "Disable terminal UI (useful with --web-port only)")

	root.AddCommand(newVersionCmd())

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pbmon %s (commit %s)\n", version.VERSION, version.COMMIT)
		},
	}
}

func run(cfg *config.Config, logPath string, noTUI bool) error {
	logger := buildLogger(cfg.Debug, logPath)

	// Kernel version check
	if err := checkKernelVersion(4, 9, 0); err != nil {
		return err
	}

	// Context tied to OS signals for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create and start the collector (loads eBPF)
	coll, err := collector.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("initialize collector: %w", err)
	}
	coll.Start()

	var wg sync.WaitGroup

	// Start web server if configured
	if cfg.WebPort > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := web.Start(ctx, cfg, coll.Store()); err != nil {
				logger.Error("web server error", "err", err)
			}
		}()
	}

	// Start TUI unless disabled
	if !noTUI {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := tui.Start(ctx, stop, cfg, coll.Store()); err != nil {
				logger.Error("TUI error", "err", err)
			}
		}()
	}

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutting down…")

	// Stop collector first (no new BPF events), then wait for UI/server
	coll.Stop()
	wg.Wait()

	logger.Info("Goodbye.")
	return nil
}

// buildLogger creates a slog.Logger writing to stderr (or logPath if set).
func buildLogger(debug bool, logPath string) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	out := os.Stderr
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			out = f
		}
	}

	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: level}))
}

// checkKernelVersion reads /proc/version and compares against the minimum.
func checkKernelVersion(major, minor, patch int) error {
	f, err := os.Open("/proc/version")
	if err != nil {
		return nil // can't check; proceed
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Scan()
	line := sc.Text()

	// Format: "Linux version X.Y.Z-... (gcc ...) ..."
	var kmaj, kmin, kpat int
	// Find "version X.Y.Z"
	idx := strings.Index(line, "version ")
	if idx < 0 {
		return nil
	}
	parts := strings.Fields(line[idx+len("version "):])
	if len(parts) == 0 {
		return nil
	}
	_, _ = fmt.Sscanf(strings.SplitN(parts[0], "-", 2)[0], "%d.%d.%d", &kmaj, &kmin, &kpat)

	needed := major*10000 + minor*100 + patch
	have := kmaj*10000 + kmin*100 + kpat

	if have > 0 && have < needed {
		return fmt.Errorf("Linux kernel %d.%d.%d is not supported (need >= %d.%d.%d)",
			kmaj, kmin, kpat, major, minor, patch)
	}
	return nil
}

