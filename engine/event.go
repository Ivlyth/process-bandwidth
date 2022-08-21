package engine

import (
	"bytes"
	"encoding/binary"
	"github.com/Ivlyth/process-bandwidth/pkg/sync"
	"github.com/shirou/gopsutil/v3/process"
	"strings"
)

type Direction uint8

func (d Direction) String() string {
	if d == 0 {
		return "write"
	} else {
		return "read"
	}
}

type FDType uint8

func (t FDType) String() string {
	if t == 0 {
		return "unknown"
	} else if t == 1 {
		return "socket"
	} else {
		return "file"
	}
}

const DirectionOutgoing Direction = 0
const DirectionIncoming Direction = 1

const FDTypeUnknown FDType = 0
const FDTypeSocket FDType = 1
const FDTypeFile FDType = 2

type EventType uint8

const EventTypeRW EventType = 0
const EventTypeClose EventType = 1
const EventTypeExit EventType = 2

type Event interface {
	Decode(buf *bytes.Buffer) error
	Process()
}

type CloseEvent struct {
	Pid uint32
	Tid uint32
	FD  uint32

	bp     *Process
	bc     *Connection
	bci    *ConnectionInfo
	loaded bool
}

func (e *CloseEvent) Decode(buf *bytes.Buffer) (err error) {
	if err = binary.Read(buf, endian, &e.Pid); err != nil {
		return
	}
	if err = binary.Read(buf, endian, &e.Tid); err != nil {
		return
	}
	if err = binary.Read(buf, endian, &e.FD); err != nil {
		return
	}

	return nil
}

func (e *CloseEvent) Process() {
	e.bp, e.loaded = ProcessesMap.Load(e.Pid)
	if e.loaded {
		e.bc, e.loaded = e.bp.Connections.LoadAndDelete(e.FD)
		if e.loaded {
			if e.bci = e.bc.ConnectionInfo; e.bci != nil {
				ConnectionsMap.Delete(e.bci.Inode)
			}
		}
	}
}

type ExitEvent struct {
	Pid uint32
	Tid uint32
}

func (e *ExitEvent) Decode(buf *bytes.Buffer) (err error) {
	if err = binary.Read(buf, endian, &e.Pid); err != nil {
		return
	}
	if err = binary.Read(buf, endian, &e.Tid); err != nil {
		return
	}

	return nil
}

func (e *ExitEvent) Process() {
	if e.Pid != e.Tid { // we only care about the main thread(thread group leader)
		return
	}
	ProcessesMap.Delete(e.Pid)
}

type RWEvent struct {
	Pid       uint32
	Tid       uint32
	FD        uint32
	FDType    FDType    // 0-unknown, 1-socket, 2-file and others
	Direction Direction // 0-outgoing, 1-incoming
	Size      uint64

	bp  *Process
	bc  *Connection
	bci *ConnectionInfo

	p       *process.Process
	name    string
	cmdline []string
	err     error

	iface  *Interface
	loaded bool
}

func (e *RWEvent) Decode(buf *bytes.Buffer) (err error) {
	if err = binary.Read(buf, endian, &e.Pid); err != nil {
		return
	}
	if err = binary.Read(buf, endian, &e.Tid); err != nil {
		return
	}
	if err = binary.Read(buf, endian, &e.FD); err != nil {
		return
	}
	if err = binary.Read(buf, endian, &e.FDType); err != nil {
		return
	}
	if err = binary.Read(buf, endian, &e.Direction); err != nil {
		return
	}
	if err = binary.Read(buf, endian, &e.Size); err != nil {
		return
	}

	return nil
}

func (e *RWEvent) Process() {

	e.bp, e.loaded = ProcessesMap.LoadOrStore(e.Pid, func() *Process {
		return &Process{
			Pid:         e.Pid,
			Name:        "unknown",
			Connections: &sync.Map[uint32, *Connection]{},
		}
	})

	e.bp.Init(func() {
		// try to get the process
		e.p, e.err = process.NewProcess(int32(e.bp.Pid))
		if e.err == nil {
			e.bp.process = e.p

			e.name, e.err = e.p.Name()
			if e.err == nil && len(e.name) > 0 {
				e.bp.Name = e.name
			}

			e.cmdline, e.err = e.p.CmdlineSlice()
			if e.err == nil && len(e.cmdline) > 0 {
				e.bp.Cmdline = strings.Join(cleanCmdline(e.cmdline), " ")
			}
		}
	})

	if e.bp.process == nil {
		// keep the reference in the map for latter filter to improve performance
		return
	}

	e.bc, _ = e.bp.Connections.LoadOrStore(e.FD, func() *Connection {
		return &Connection{
			FD:     e.FD,
			FDType: e.FDType,
		}
	})

	e.bc.Init(func() {
		if e.bc.FDType == FDTypeUnknown || e.bc.FDType == FDTypeSocket {
			e.bci = getConnectionInfoFor(e.Pid, e.Tid, e.FD)

			if e.bci != nil {
				e.bc.FD = e.FD
				e.bc.FDType = FDTypeSocket
				e.bc.INode = e.bci.Inode
				e.bc.ConnectionInfo = e.bci
			}
		}
		//else if e.bc.FDType == FDTypeFile {
		//	// TODO 可能和上边的 get 合并逻辑做快速查找
		//}
	})

	if e.bc.ShouldSkip() {
		return
	}

	e.bc.addByRWEvent(e)

	e.bp.addByRWEvent(e)

	e.iface, e.loaded = interfacesMap.Load(e.bc.ConnectionInfo.LocalIP)
	if e.loaded {
		e.bc.IfName = e.iface.Name
		e.iface.addByRWEvent(e)
	} else {
		e.bc.IfName = "unknown"
	}
}

func cleanCmdline(cmdline []string) []string {
	newCmdline := make([]string, 0, len(cmdline))
	for _, s := range cmdline {
		ts := strings.TrimSpace(s)
		if ts == "" {
			continue
		}
		newCmdline = append(newCmdline, ts)
	}
	return newCmdline
}
