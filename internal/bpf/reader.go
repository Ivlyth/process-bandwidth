package bpf

import (
	"errors"
	"io"
	"os"

	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/ringbuf"
)

// EventReader is a unified interface over both ringbuf.Reader and perf.Reader.
// Read() blocks until an event is available or the reader is closed.
type EventReader interface {
	Read() ([]byte, error)
	Close() error
}

// NewReader creates the appropriate EventReader based on what was loaded.
func NewReader(objs *Objects) (EventReader, error) {
	if objs.UseRingBuf() {
		r, err := ringbuf.NewReader(objs.Events)
		if err != nil {
			return nil, err
		}
		return &ringBufReader{r: r}, nil
	}
	// Perf event array: use 256 pages (~1 MB) per CPU.
	r, err := perf.NewReader(objs.Events, os.Getpagesize()*256)
	if err != nil {
		return nil, err
	}
	return &perfEventReader{r: r}, nil
}

// ──────────────────────────────────────────────────────────────
// Ring buffer reader
// ──────────────────────────────────────────────────────────────

type ringBufReader struct {
	r *ringbuf.Reader
}

func (r *ringBufReader) Read() ([]byte, error) {
	rec, err := r.r.Read()
	if err != nil {
		if errors.Is(err, ringbuf.ErrClosed) {
			return nil, io.EOF
		}
		return nil, err
	}
	return rec.RawSample, nil
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
			// Caller can track drops via a separate counter if needed.
			// Return a sentinel so the collector can count dropped events.
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
