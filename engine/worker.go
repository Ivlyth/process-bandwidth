package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/Ivlyth/process-bandwidth/config"
	"github.com/Ivlyth/process-bandwidth/logging"
	"github.com/Ivlyth/process-bandwidth/pkg/profile"
	"go.uber.org/atomic"
	"time"
)

var logger = logging.GetLogger()
var totalCounter atomic.Uint64
var errCounter atomic.Uint64

func startWorkers(c <-chan []byte, errChan chan<- error) {
	err := initInterfaces()
	if err != nil {
		errChan <- err
		return
	}
	EventProcessProfileCountersMap = make(map[string]*profile.Counter, config.GlobalConfig.WorkersCount)
	for i := 0; i < config.GlobalConfig.WorkersCount; i++ {
		pc := &profile.Counter{}
		EventProcessProfileCountersMap[fmt.Sprintf("worker-%d", i)] = pc
		go processPerfEvents(c, pc)
	}
	go startSnapShotWorker()
}

func startSnapShotWorker() {
	ticker := time.Tick(time.Second)

	var start, end time.Time
	var duration time.Duration

	for {
		select {
		case <-ticker:
			start = time.Now()
			SnapshotLock.Lock()
			ProcessesMap.Range(func(pid uint32, bp *Process) bool {
				if bp.isIdle() { // idle process
					ProcessesMap.Delete(bp.Pid)
					return true
				}

				account := 0
				bp.Connections.Range(func(fd uint32, bc *Connection) bool {
					if bc.isIdle() { // idle connection
						bp.Connections.Delete(bc.FD)
						if ci := bc.ConnectionInfo; ci != nil {
							ConnectionsMap.Delete(ci.Inode)
						}
						return true
					}

					if bc.ShouldSkip() {
						return true
					}

					account++
					bc.takeSnapshot()

					return true
				})

				if account == 0 {
					return true
				}
				bp.takeSnapshot()
				return true
			})

			// interfaceMap just for debug use, after all so many tools can offer this data and better than us
			interfacesMap.Range(func(name string, bi *Interface) bool {
				bi.takeSnapshot()
				return true
			})
			SnapshotLock.Unlock()

			end = time.Now()
			duration = end.Sub(start)
			SnapShotProfileCounter.Inc(duration)
		}
	}
}

func processPerfEvents(c <-chan []byte, pc *profile.Counter) {

	rwEvent := &RWEvent{}
	closeEvent := &CloseEvent{}
	exitEvent := &ExitEvent{}
	var event Event
	var eventType EventType

	var start time.Time

	for sample := range c {
		start = time.Now()
		totalCounter.Inc()

		buf := bytes.NewBuffer(sample)

		if err := binary.Read(buf, binary.LittleEndian, &eventType); err != nil {
			continue
		}

		switch eventType {
		case EventTypeClose:
			event = closeEvent
		case EventTypeExit:
			event = exitEvent
		case EventTypeRW:
			event = rwEvent
		default:
			logger.Printf("ERROR unknown event type value: %x\n", eventType)
			continue
		}

		err := event.Decode(buf)
		if err != nil {
			errCounter.Inc()
			continue
		}

		event.Process()
		pc.Inc(time.Now().Sub(start))
	}
}
