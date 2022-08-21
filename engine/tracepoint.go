package engine

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	manager "github.com/ehids/ebpfmanager"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"math"
	"os"
	"strings"
	"time"
)

//go:embed tracepoint.o
var TracePointELFContent []byte

type TracePointProbe struct {
	bpfManager        *manager.Manager
	bpfManagerOptions manager.Options
	eventMap          *ebpf.Map
}

func (tp *TracePointProbe) start() error {

	err := tp.setupManagers()
	if err != nil {
		return errors.Wrap(err, "load ebpf program failed")
	}

	if err = tp.bpfManager.Start(); err != nil {
		return errors.Wrap(err, "bootstrap ebpf program failed")
	}

	err = tp.findEventMap()
	if err != nil {
		return err
	}

	return nil
}

// FIXME use me !!!
func (tp *TracePointProbe) Close() error {
	if err := tp.bpfManager.Stop(manager.CleanAll); err != nil {
		return errors.Wrap(err, "couldn't stop manager")
	}
	return nil
}

var tracepoints = []string{
	"sys_exit_accept",
	"sys_exit_accept4",
	"sys_exit_socket",

	"sys_exit_open",
	"sys_exit_openat",
	"sys_exit_creat",

	"sys_enter_write",
	"sys_exit_write",
	"sys_enter_read",
	"sys_exit_read",

	"sys_enter_sendto",
	"sys_exit_sendto",
	"sys_enter_sendmsg",
	"sys_exit_sendmsg",
	"sys_enter_sendmmsg",
	"sys_exit_sendmmsg",
	"sys_enter_sendfile64",
	"sys_exit_sendfile64",

	"sys_enter_recvfrom",
	"sys_exit_recvfrom",
	"sys_enter_recvmsg",
	"sys_exit_recvmsg",
	"sys_enter_recvmmsg",
	"sys_exit_recvmmsg",

	"sys_enter_close",
	"sched/sched_process_exit",
}

func (tp *TracePointProbe) setupManagers() error {
	var probes []*manager.Probe
	for _, tpName := range tracepoints {
		category := "syscalls"
		funcName := tpName
		parts := strings.Split(tpName, "/")
		if len(parts) == 2 {
			category = parts[0]
			funcName = parts[1]
		}
		probes = append(probes, &manager.Probe{
			Section:      fmt.Sprintf("tracepoint/%s/%s", category, funcName),
			EbpfFuncName: fmt.Sprintf("tracepoint_%s", funcName),
		})
	}
	tp.bpfManager = &manager.Manager{
		Probes: probes,
		Maps: []*manager.Map{
			{
				Name: "bp_events",
			},
		},
	}

	tp.bpfManagerOptions = manager.Options{
		DefaultKProbeMaxActive: 1024,
		VerifierOptions: ebpf.CollectionOptions{
			Programs: ebpf.ProgramOptions{
				LogSize: 2097152,
			},
		},
		RLimit: &unix.Rlimit{
			Cur: math.MaxUint64,
			Max: math.MaxUint64,
		},
	}

	if len(TracePointELFContent) == 0 {
		return errors.New("TracePointELFContent is empty")
	}

	if err := tp.bpfManager.InitWithOptions(bytes.NewReader(TracePointELFContent), tp.bpfManagerOptions); err != nil {
		return errors.Wrap(err, "couldn't init manager")
	}

	return nil
}

func (tp *TracePointProbe) findEventMap() error {
	BPEventMap, found, err := tp.bpfManager.GetMap("bp_events")
	if err != nil {
		return err
	}
	if !found {
		return errors.New("cant found map:bp_events")
	}
	tp.eventMap = BPEventMap

	return nil
}

func (tp *TracePointProbe) startPerfEventReader(c chan<- []byte, errChan chan<- error) {
	rd, err := perf.NewReader(tp.eventMap, os.Getpagesize()*1024)
	if err != nil {
		errChan <- errors.Wrap(err, fmt.Sprintf("create perf reader for map `%s` failed", tp.eventMap.String()))
		return
	}
	defer func(rd *perf.Reader) {
		_ = rd.Close()
	}(rd)

	var record perf.Record
	var now time.Time

	for {
		//判断ctx是不是结束
		//select {
		//case _ = <-tp.ctx.Done():
		//	log.Printf("readEvent recived close signal from context.Done.")
		//	return
		//default:
		//}

		now = time.Now()

		record, err = rd.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			errChan <- fmt.Errorf("ERROR: reading from perf event reader: %s", err)
			return
		}

		if record.LostSamples != 0 {
			PerfReaderLostProfileCounter.Add(record.LostSamples, time.Now().Sub(now))
			PerfReaderReadProfileCounter.Add(record.LostSamples, time.Now().Sub(now))
			logger.Printf("ERROR: perf event ring buffer full - %s, dropped %d samples", tp.eventMap.String(), record.LostSamples)
			continue
		}

		c <- record.RawSample
		PerfReaderReadProfileCounter.Inc(time.Now().Sub(now))
	}
}
