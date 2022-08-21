package prom

type AccumulatorExtension struct {
	acc Accumulator
}

func NewAccumulatorExtension(acc Accumulator) AccumulatorExtension {
	return AccumulatorExtension{acc}
}

//// AddCounter is the same as AddFields, but will add the metric as a "Counter" type
//AddCounter(measurement string,
//	fields map[string]interface{},
//	tags map[string]string,
//	t ...time.Time)
//
//// AddSummary is the same as AddFields, but will add the metric as a "Summary" type
//AddSummary(measurement string,
//	fields map[string]interface{},
//	tags map[string]string,
//	t ...time.Time)
//
//// AddHistogram is the same as AddFields, but will add the metric as a "Histogram" type
//AddHistogram(measurement string,
//	fields map[string]interface{},
//	tags map[string]string,
//	t ...time.Time)

//// AddGauge is the same as AddFields, but will add the metric as a "Gauge" type
//AddGauge(measurement string,
//fields map[string]interface{},
//tags map[string]string,
//t ...time.Time)
