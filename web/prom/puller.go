package prom

import (
	"github.com/Ivlyth/process-bandwidth/logging"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var logger = logging.GetLogger()

type PullSource interface {
	// Gather 用于收集需要的数据信息
	Gather(Accumulator) error
	Name() string
}

// Puller 提供 prometheus.Collector 实现，用于从一组 collector.Gather 收集数据
type Puller struct {
	pullSources []PullSource
}

var _ prometheus.Collector = (*Puller)(nil)

func NewPuller() *Puller {
	return &Puller{}
}

func (m *Puller) Add(sources ...PullSource) {
	m.pullSources = append(m.pullSources, sources...)
}

func (m *Puller) Describe(descs chan<- *prometheus.Desc) {
	// Sending no descriptor at all marks the Collector as "unchecked",
	// i.e. no checks will be performed at registration time, and the
	// Puller may yield any Metric it sees fit in its Collect method.
}

func (m *Puller) Collect(out chan<- prometheus.Metric) {
	generalCh := make(chan Metric, 10)
	acc := NewAccumulator(nil, generalCh)

	go func() {
		wg := sync.WaitGroup{}
		for _, ps := range m.pullSources {
			wg.Add(1)
			go func(ps PullSource) {
				defer wg.Done()

				err := ps.Gather(acc)
				if err != nil {
					logger.Debugf("Error when gather metrics from %s: %s", ps.Name(), err.Error())
				}
			}(ps)
		}
		wg.Wait()

		close(generalCh)
	}()

	for data := range generalCh {
		for _, m := range Metrics(data) {
			out <- m
		}
	}
}
