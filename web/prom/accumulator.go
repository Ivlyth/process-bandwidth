package prom

import "time"

type Accumulator interface {
	// AddFields 添加一组值
	AddFields(name string, tags []*Tag, fields map[string]TypedValue, tm time.Time)

	// AddMetric adds an metric to the accumulator.
	AddMetric(Metric)

	// SetPrecision sets the timestamp rounding precision. All metrics
	// added to the accumulator will have their timestamp rounded to the
	// nearest multiple of precision.
	SetPrecision(precision time.Duration)

	//// Report an error.
	//AddError(err error)

	// Extension 返回 Accumulator 的通用扩展，实现方只需简单返回对 NewAccumulatorExtension 的调用即可
	Extension() AccumulatorExtension
}
