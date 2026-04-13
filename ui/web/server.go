// Package web provides the HTTP server for pbmon:
//   - REST API  (/api/processes, /api/overview)
//   - WebSocket (/ws) for real-time updates
//   - Prometheus metrics (/metrics or configurable path)
//   - Embedded static web dashboard (/)
package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Ivlyth/process-bandwidth/config"
	"github.com/Ivlyth/process-bandwidth/internal/store"
)

// Start launches the HTTP server and blocks until ctx is cancelled or a
// fatal error occurs. Shutdown is graceful with a 5-second drain timeout.
func Start(ctx context.Context, cfg *config.Config, s *store.Store) error {
	logger := slog.Default()

	// Prometheus registry (isolated – don't use the global default registry)
	reg := prometheus.NewRegistry()
	reg.MustRegister(newPbmonCollector(s))

	// WebSocket hub
	hub := newWSHub(s, logger)
	go hub.Run(ctx)

	// HTTP mux
	mux := http.NewServeMux()

	// REST API
	mux.HandleFunc("/api/processes", apiProcesses(s))
	mux.HandleFunc("/api/overview", apiOverview(s))

	// WebSocket
	mux.Handle("/ws", hub)

	// Prometheus metrics
	metricsPath := cfg.MetricsPath
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	mux.Handle(metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	// Static files (embedded SPA)
	staticHandler := http.FileServer(http.FS(staticFS))
	mux.Handle("/", staticHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.WebPort),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("web server starting", "addr", srv.Addr)

	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger.Info("web server shutting down")
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return fmt.Errorf("web server: %w", err)
	}
}
