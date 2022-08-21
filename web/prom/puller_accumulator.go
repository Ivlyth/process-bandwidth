package prom

import (
	"time"
)

type pleaseInputNil interface {
	doNotImplementThisInterface()
}

// accumulator 提供了 collector.Gather 的实现，用于收集所有的数据并以 Channel 的形式写出
type accumulator struct {
	out       chan<- Metric
	precision time.Duration
}

func NewAccumulator(x pleaseInputNil, out chan<- Metric) Accumulator {
	acc := &accumulator{
		out:       out,
		precision: time.Nanosecond,
	}
	return acc
}

func (ac *accumulator) SetPrecision(precision time.Duration) {
	ac.precision = precision
}

func (ac *accumulator) AddFields(name string, tags []*Tag, fields map[string]TypedValue, tm time.Time) {
	m := NewMetric(name, tags, fields, tm)
	ac.AddMetric(m)
}

func (ac *accumulator) AddMetric(m Metric) {
	m.SetTime(m.Time().Round(ac.precision))
	if m := ac.simpleMakeMetric(m); m != nil {
		ac.out <- m
	}
}

func (ac *accumulator) getTime(t []time.Time) time.Time {
	var timestamp time.Time
	if len(t) > 0 {
		timestamp = t[0]
	} else {
		timestamp = time.Now()
	}
	return timestamp.Round(ac.precision)
}

// simpleMakeMetric 不做真正的 metric 过滤/转换
func (ac *accumulator) simpleMakeMetric(m Metric) Metric {
	return m
}

func (ac *accumulator) Extension() AccumulatorExtension {
	return NewAccumulatorExtension(ac)
}
