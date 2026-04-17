package collector

import (
	"encoding/binary"
	"testing"
)

// buildIOEvent constructs a binary payload matching the packed C struct io_event_t.
func buildIOEvent(pid, tid, fd uint32, fdClass, direction uint8, bytes uint64) []byte {
	// Layout: u8 ev_type, u32 pid, u32 tid, u32 fd, u8 fd_class, u8 direction, u64 bytes
	// Total: 1+4+4+4+1+1+8 = 23 bytes
	buf := make([]byte, 23)
	buf[0] = evIO
	binary.LittleEndian.PutUint32(buf[1:5], pid)
	binary.LittleEndian.PutUint32(buf[5:9], tid)
	binary.LittleEndian.PutUint32(buf[9:13], fd)
	buf[13] = fdClass
	buf[14] = direction
	binary.LittleEndian.PutUint64(buf[15:23], bytes)
	return buf
}

// buildFDEvent constructs a binary payload for fd_event_t.
func buildFDEvent(pid, tid, fd uint32, fdClass, op uint8) []byte {
	// Layout: u8 ev_type, u32 pid, u32 tid, u32 fd, u8 fd_class, u8 op
	// Total: 1+4+4+4+1+1 = 15 bytes
	buf := make([]byte, 15)
	buf[0] = evFD
	binary.LittleEndian.PutUint32(buf[1:5], pid)
	binary.LittleEndian.PutUint32(buf[5:9], tid)
	binary.LittleEndian.PutUint32(buf[9:13], fd)
	buf[13] = fdClass
	buf[14] = op
	return buf
}

// buildProcEvent constructs a binary payload for proc_event_t.
func buildProcEvent(pid, tid uint32, op uint8) []byte {
	// Layout: u8 ev_type, u32 pid, u32 tid, u8 op
	// Total: 1+4+4+1 = 10 bytes
	buf := make([]byte, 10)
	buf[0] = evProc
	binary.LittleEndian.PutUint32(buf[1:5], pid)
	binary.LittleEndian.PutUint32(buf[5:9], tid)
	buf[9] = op
	return buf
}

func TestDecodeIOEvent(t *testing.T) {
	raw := buildIOEvent(1234, 5678, 42, 1 /*socket*/, dirRead, 8192)
	ev, err := DecodeEvent(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	io, ok := ev.(*IOEvent)
	if !ok {
		t.Fatalf("expected *IOEvent, got %T", ev)
	}
	if io.PID != 1234 {
		t.Errorf("PID: want 1234, got %d", io.PID)
	}
	if io.TID != 5678 {
		t.Errorf("TID: want 5678, got %d", io.TID)
	}
	if io.FD != 42 {
		t.Errorf("FD: want 42, got %d", io.FD)
	}
	if io.FDClass != 1 {
		t.Errorf("FDClass: want 1, got %d", io.FDClass)
	}
	if io.Direction != dirRead {
		t.Errorf("Direction: want dirRead, got %d", io.Direction)
	}
	if io.Bytes != 8192 {
		t.Errorf("Bytes: want 8192, got %d", io.Bytes)
	}
}

func TestDecodeFDEvent(t *testing.T) {
	raw := buildFDEvent(100, 101, 7, 2 /*file*/, fdOpClose)
	ev, err := DecodeEvent(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fd, ok := ev.(*FDEvent)
	if !ok {
		t.Fatalf("expected *FDEvent, got %T", ev)
	}
	if fd.PID != 100 {
		t.Errorf("PID: want 100, got %d", fd.PID)
	}
	if fd.Op != fdOpClose {
		t.Errorf("Op: want fdOpClose, got %d", fd.Op)
	}
}

func TestDecodeProcEvent(t *testing.T) {
	raw := buildProcEvent(9999, 9999, procOpExit)
	ev, err := DecodeEvent(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pe, ok := ev.(*ProcEvent)
	if !ok {
		t.Fatalf("expected *ProcEvent, got %T", ev)
	}
	if pe.PID != 9999 {
		t.Errorf("PID: want 9999, got %d", pe.PID)
	}
	if pe.Op != procOpExit {
		t.Errorf("Op: want procOpExit, got %d", pe.Op)
	}
}

func TestDecodeEventTooShort(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
	}{
		{"io too short", []byte{evIO, 0, 0}},
		{"fd too short", []byte{evFD, 0}},
		{"proc too short", []byte{evProc, 1, 2}},
		{"empty", []byte{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeEvent(tc.raw)
			if err == nil {
				t.Error("expected error for short payload, got nil")
			}
		})
	}
}

func TestDecodeUnknownType(t *testing.T) {
	_, err := DecodeEvent([]byte{255, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}
