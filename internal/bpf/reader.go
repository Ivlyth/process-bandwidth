package bpf

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/ringbuf"
)

// EventReader abstracts the ring-buffer or perf-event-array event reader.
// Read() blocks until an event is available or the reader is closed.
type EventReader interface {
	Read() ([]byte, error)
	Close() error
}

// NewReader creates an event reader, choosing ring-buffer or perf based on the
// map type that was loaded.
func NewReader(objs *Objects) (EventReader, error) {
	return NewReaderWithLogger(objs, slog.Default())
}

// NewReaderWithLogger creates an event reader with diagnostic logging.
func NewReaderWithLogger(objs *Objects, logger *slog.Logger) (EventReader, error) {
	mapType := objs.Events.Type()
	maxEntries := objs.Events.MaxEntries()
	logger.Debug("events map", "type", mapType, "max_entries", maxEntries)

	switch mapType {
	case ebpf.RingBuf:
		logger.Debug("using BPF ring buffer reader (kernel >= 5.8)")
		r, err := ringbuf.NewReader(objs.Events)
		if err != nil {
			return nil, fmt.Errorf("ringbuf reader: %w", err)
		}
		logger.Debug("ring buffer reader created")
		return &ringBufReader{r: r}, nil

	default:
		// PERF_EVENT_ARRAY path (kernel 4.9+).
		if maxEntries == 0 {
			return nil, fmt.Errorf(
				"events map MaxEntries=0: perf reader would create no CPU rings; " +
					"re-run `make ebpf && go build` to embed a fresh BPF object")
		}
		pageSize := os.Getpagesize()
		bufSize := pageSize * 256
		logger.Debug("using perf event array reader",
			"map_type", mapType,
			"max_entries", maxEntries,
			"per_cpu_buf_bytes", bufSize,
		)
		r, err := perf.NewReader(objs.Events, bufSize)
		if err != nil {
			return nil, fmt.Errorf("perf reader: %w", err)
		}
		logger.Debug("perf event reader created", "cpu_rings", maxEntries)
		return &perfEventReader{r: r}, nil
	}
}

// ──────────────────────────────────────────────────────────────
// Ring buffer reader
// ──────────────────────────────────────────────────────────────

type ringBufReader struct {
	r *ringbuf.Reader
}

func (r *ringBufReader) Read() ([]byte, error) {
	for {
		rec, err := r.r.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return nil, io.EOF
			}
			return nil, err
		}
		return rec.RawSample, nil
	}
}

func (r *ringBufReader) Close() error {
	return r.r.Close()
}

// ──────────────────────────────────────────────────────────────
// Perf event array reader
// ──────────────────────────────────────────────────────────────

type perfEventReader struct {
	r *perf.Reader
}

func (r *perfEventReader) Read() ([]byte, error) {
	for {
		rec, err := r.r.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return nil, io.EOF
			}
			return nil, err
		}
		if rec.LostSamples > 0 {
			return nil, &DroppedSamplesError{Count: rec.LostSamples}
		}
		return rec.RawSample, nil
	}
}

func (r *perfEventReader) Close() error {
	return r.r.Close()
}

// DroppedSamplesError is returned when the perf ring buffer overflows.
type DroppedSamplesError struct {
	Count uint64
}

func (e *DroppedSamplesError) Error() string {
	return "perf ring buffer overflow"
}
