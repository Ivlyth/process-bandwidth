package engine

import (
	"github.com/Ivlyth/process-bandwidth/pkg/sync"
	"github.com/shirou/gopsutil/v3/process"
)

// 从 events 收到消息后, 不断进行累加, 另一个 goroutine 每秒去对其进行遍历计算差值并打印

var ProcessesMap = &sync.Map[uint32, *Process]{}

type Process struct {
	Pid     uint32
	Name    string
	Cmdline string

	process *process.Process

	connOnce    sync.Once // 用于首次初始化 Connections Map
	Connections *sync.Map[uint32, *Connection]

	RWEventCounter
}

func (p *Process) GetConnections() []*Connection {
	conns := []*Connection{}
	p.Connections.Range(func(fd uint32, bc *Connection) bool {
		if bc.ShouldSkip() {
			return true
		}
		conns = append(conns, bc)
		return true
	})
	return conns
}

func (p *Process) DynName() string {
	if p.process != nil {
		parts, err := p.process.CmdlineSlice()
		if err == nil && len(parts) > 0 {
			return parts[0]
		}
	}
	return p.Name
}
