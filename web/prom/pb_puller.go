package prom

import (
	"github.com/Ivlyth/process-bandwidth/engine"
	"os"
	"strconv"
	"sync"
	"time"
)

type ProcessBandwidthPuller struct {
	mu sync.Mutex
}

var _ PullSource = (*ProcessBandwidthPuller)(nil)

func NewProcessBandwidthPuller() *ProcessBandwidthPuller {
	return &ProcessBandwidthPuller{}
}

func (x *ProcessBandwidthPuller) Name() string {
	return "ProcessBandwidthPuller"
}

func (x *ProcessBandwidthPuller) Gather(acc Accumulator) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "<error>"
	}

	now := time.Now()

	engine.ProcessesMap.Range(func(pid uint32, bp *engine.Process) bool {
		s := bp.LastSnapshot()
		tags := []*Tag{
			NewTag("pid", strconv.Itoa(int(bp.Pid))),
			NewTag("name", bp.Name),
			NewTag("hostname", hostname),
		}
		fields := map[string]TypedValue{
			"in":  NewGaugeValue(s.IncomingRatebps()),
			"out": NewGaugeValue(s.OutgoingRatebps()),
		}
		acc.AddFields("process_bandwidth", tags, fields, now)

		bp.Connections.Range(func(fd uint32, bc *engine.Connection) bool {
			if bc.ShouldSkip() {
				return true
			}
			s := bc.LastSnapshot()
			tags := []*Tag{
				NewTag("sip", bc.ConnectionInfo.LocalIP),
				NewTag("sport", strconv.Itoa(int(bc.ConnectionInfo.LocalPort))),
				NewTag("dip", bc.ConnectionInfo.RemoteIP),
				NewTag("dport", strconv.Itoa(int(bc.ConnectionInfo.RemotePort))),
				NewTag("hostname", hostname),
			}
			fields := map[string]TypedValue{
				"in":  NewGaugeValue(s.IncomingRatebps()),
				"out": NewGaugeValue(s.OutgoingRatebps()),
			}
			acc.AddFields("process_bandwidth_connection", tags, fields, now)
			return true
		})
		return true
	})

	logger.Debugf("metrics interface use %s", time.Now().Sub(now))
	return nil
}
