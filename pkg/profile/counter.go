package profile

import (
	"fmt"
	"time"
)

type Counter struct {
	Count    uint64
	Duration time.Duration
}

func (pc *Counter) AvgDuration() string {
	if pc.Count == 0 {
		return "<no data>"
	}
	return (pc.Duration / time.Duration(pc.Count)).String()
}

func (pc *Counter) AvgRate() string {
	if pc.Duration == 0 {
		return "<no data>"
	}
	return fmt.Sprintf("%.2f/s", float64(pc.Count)/(float64(pc.Duration)/float64(time.Second)))
}

func (pc *Counter) String() string {
	if pc.Count == 0 {
		return "<no data>"
	}
	return fmt.Sprintf("count: %d, rate: %s, avg cost: %s", pc.Count, pc.AvgRate(), pc.AvgDuration())
}

func (pc *Counter) Inc(d time.Duration) {
	pc.Add(1, d)
}

func (pc *Counter) Add(n uint64, d time.Duration) {
	pc.Count += n
	pc.Duration += d
}
