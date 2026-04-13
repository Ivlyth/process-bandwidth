package bpf

import (
	"errors"
	"io"
	"os"

	"github.com/cilium/ebpf/perf"
)

// EventReader abstracts the perf event reader.
// Read() blocks until an event is available or the reader is closed.
type EventReader interface {
	Read() ([]byte, error)
	Close() error
}

// NewReader creates a perf-event-array reader for the events map.
// Uses 256 pages (~1 MB) per CPU as the per-CPU ring buffer size.
func NewReader(objs *Objects) (EventReader, error) {
	r, err := perf.NewReader(objs.Events, os.Getpagesize()*256)
	if err != nil {
		return nil, err
	}
	return &perfEventReader{r: r}, nil
}

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

