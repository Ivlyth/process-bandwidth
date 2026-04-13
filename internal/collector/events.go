package collector

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Event type constants – must match the C definitions in pbmon.c.
const (
	evIO   uint8 = 0
	evFD   uint8 = 1
	evProc uint8 = 2
)

// Direction constants – must match DIR_WRITE / DIR_READ in pbmon.c.
const (
	dirWrite uint8 = 0
	dirRead  uint8 = 1
)

// FD operation constants – must match FD_OP_* in pbmon.c.
const (
	fdOpOpen  uint8 = 0
	fdOpClose uint8 = 1
)

// Process operation constants – must match PROC_OP_* in pbmon.c.
const (
	procOpExit uint8 = 0
	procOpFork uint8 = 1
)

// IOEvent represents a kernel read/write syscall event.
// Must match the packed C struct io_event_t exactly:
//
//	u8  ev_type
//	u32 pid
//	u32 tid
//	u32 fd
//	u8  fd_class
//	u8  direction
//	u64 bytes
type IOEvent struct {
	EvType    uint8
	PID       uint32
	TID       uint32
	FD        uint32
	FDClass   uint8
	Direction uint8
	Bytes     uint64
}

// FDEvent represents an fd lifecycle event (open/close/dup).
// Must match the packed C struct fd_event_t:
//
//	u8  ev_type
//	u32 pid
//	u32 tid
//	u32 fd
//	u8  fd_class
//	u8  op
type FDEvent struct {
	EvType  uint8
	PID     uint32
	TID     uint32
	FD      uint32
	FDClass uint8
	Op      uint8
}

// ProcEvent represents a process lifecycle event (exit/fork).
// Must match the packed C struct proc_event_t:
//
//	u8  ev_type
//	u32 pid
//	u32 tid
//	u8  op
type ProcEvent struct {
	EvType uint8
	PID    uint32
	TID    uint32
	Op     uint8
}

// DecodeEvent inspects the first byte (ev_type) of raw and decodes the rest
// into the appropriate struct. Returns one of *IOEvent, *FDEvent, *ProcEvent.
func DecodeEvent(raw []byte) (any, error) {
	if len(raw) == 0 {
		return nil, io.ErrUnexpectedEOF
	}

	le := binary.LittleEndian
	switch raw[0] {
	case evIO:
		// Total packed size: 1 + 4 + 4 + 4 + 1 + 1 + 8 = 23 bytes
		if len(raw) < 23 {
			return nil, fmt.Errorf("io_event too short: %d bytes", len(raw))
		}
		ev := &IOEvent{
			EvType:    raw[0],
			PID:       le.Uint32(raw[1:5]),
			TID:       le.Uint32(raw[5:9]),
			FD:        le.Uint32(raw[9:13]),
			FDClass:   raw[13],
			Direction: raw[14],
			Bytes:     le.Uint64(raw[15:23]),
		}
		return ev, nil

	case evFD:
		// Total packed size: 1 + 4 + 4 + 4 + 1 + 1 = 15 bytes
		if len(raw) < 15 {
			return nil, fmt.Errorf("fd_event too short: %d bytes", len(raw))
		}
		ev := &FDEvent{
			EvType:  raw[0],
			PID:     le.Uint32(raw[1:5]),
			TID:     le.Uint32(raw[5:9]),
			FD:      le.Uint32(raw[9:13]),
			FDClass: raw[13],
			Op:      raw[14],
		}
		return ev, nil

	case evProc:
		// Total packed size: 1 + 4 + 4 + 1 = 10 bytes
		if len(raw) < 10 {
			return nil, fmt.Errorf("proc_event too short: %d bytes", len(raw))
		}
		ev := &ProcEvent{
			EvType: raw[0],
			PID:    le.Uint32(raw[1:5]),
			TID:    le.Uint32(raw[5:9]),
			Op:     raw[9],
		}
		return ev, nil

	default:
		return nil, fmt.Errorf("unknown event type: %d", raw[0])
	}
}
