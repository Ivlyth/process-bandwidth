package engine

import (
	"github.com/Ivlyth/process-bandwidth/config"
)

func StartEngine(errChan chan<- error) {
	tp := TracePointProbe{}
	err := tp.start()
	if err != nil {
		errChan <- err
		return
	}

	eventsChan := make(chan []byte, config.GlobalConfig.ChannelSize)
	go tp.startPerfEventReader(eventsChan, errChan)
	go startWorkers(eventsChan, errChan)
	//_ = tp.Close()  // FIXME where to stop correctly ?
}
