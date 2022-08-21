package config

import "time"

type config struct {
	Debug         bool
	WorkersCount  int // 控制 worker 数量
	ChannelSize   int // 控制 perf event 与 worker 之间的 channel 大小
	SnapShotCount int // 历史数量, 默认 300
	IdleTimeout   time.Duration

	WebServerPort uint16
	ProfilePort   uint16

	//// used for filter
	//Pids    []uint32            // 要过滤的 pid 列表 // TODO
	//pidsMap map[uint32]struct{} // 实际上用该 map 来过滤 pid
	//
	//IPs   []string // TODO
	//Ports []uint16 // TODO
	//
	//LocalIps    []string // TODO
	//LocalPorts  []uint16 // TODO
	//RemoteIps   []string // TODO
	//RemotePorts []uint16 // TODO
	//
	//IncludeFileIO bool // TODO 包含文件 IO, 独立统计
	//AllowLocalNet bool // TODO 允许 127.0.0.1 的流量被统计在内  // TODO
}

var (
	GlobalConfig = config{}
)
