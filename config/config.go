package config

import "time"

// Config holds all runtime configuration for pbmon.
type Config struct {
	// Debug enables verbose logging.
	Debug bool

	// LogPath sets the log output path. Empty string means stderr.
	LogPath string

	// WorkersCount is the number of event-processing worker goroutines.
	WorkersCount int

	// ChannelSize is the buffer size of the raw-event channel between
	// the perf/ring reader and the worker goroutines.
	ChannelSize int

	// SnapshotCount is how many 1-second history samples to keep per counter.
	SnapshotCount int

	// IdleTimeout is the duration after which a process or connection with no
	// I/O activity is removed from the store.
	IdleTimeout time.Duration

	// IncludeFileIO enables tracking of regular-file and pipe I/O in addition
	// to network I/O.
	IncludeFileIO bool

	// IncludeLocal includes loopback (127.0.0.1 / ::1) traffic in network stats.
	IncludeLocal bool

	// FilterPIDs, when non-empty, restricts monitoring to these PIDs only.
	FilterPIDs []uint32

	// FilterNames, when non-empty, restricts monitoring to processes whose
	// name contains one of these strings (case-insensitive substring match).
	FilterNames []string

	// WebPort is the port for the HTTP server (REST API + WebSocket + metrics).
	// 0 disables the web server.
	WebPort uint16

	// MetricsPath is the HTTP path for the Prometheus /metrics endpoint.
	MetricsPath string

	// NetRefreshInterval is how often /proc/net/* is re-parsed to refresh the
	// inode→connection-info cache.
	NetRefreshInterval time.Duration
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	return Config{
		WorkersCount:       2,
		ChannelSize:        10000,
		SnapshotCount:      60,
		IdleTimeout:        10 * time.Second,
		WebPort:            0,
		MetricsPath:        "/metrics",
		NetRefreshInterval: 100 * time.Millisecond,
	}
}
