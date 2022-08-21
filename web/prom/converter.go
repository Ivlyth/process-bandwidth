package prom

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

type typedValueInterface interface {
	Inner() *dto.Metric
}

func Metrics(metrics Metric) []prometheus.Metric {
	converter := metricsConverter(metrics)
	fields := metrics.FieldList()

	pMetrics := make([]prometheus.Metric, len(fields))
	for i, field := range fields {
		pMetrics[i] = converter(field)
	}

	return pMetrics
}

func MetricsGenerator(metrics Metric, out chan<- prometheus.Metric) {
	converter := metricsConverter(metrics)
	fields := metrics.FieldList()

	for _, field := range fields {
		out <- converter(field)
	}
}

type metricsConverterFunc = func(field *Field) prometheus.Metric

func metricsConverter(metrics Metric) metricsConverterFunc {
	tags := metrics.TagList()

	var timestampMs *int64
	if tt := metrics.Time(); !tt.IsZero() {
		timestampMs = proto.Int64(tt.UnixNano() / int64(time.Millisecond))
	}

	labelNames := make([]string, len(tags))
	for i, tag := range tags {
		labelNames[i] = *tag.Name
	}

	return func(field *Field) prometheus.Metric {
		name := metrics.Name() + "_" + field.Key
		metric := field.TypedValue.(typedValueInterface).Inner()

		desc := prometheus.NewDesc(name, "", labelNames, nil)

		return &simplePrometheusMetric{
			desc: desc,
			m: &dto.Metric{
				Label:       tags,
				Gauge:       metric.Gauge,
				Counter:     metric.Counter,
				Summary:     metric.Summary,
				Untyped:     metric.Untyped,
				Histogram:   metric.Histogram,
				TimestampMs: timestampMs,
			},
		}
	}
}

type simplePrometheusMetric struct {
	desc *prometheus.Desc
	m    *dto.Metric
}

func (x *simplePrometheusMetric) Desc() *prometheus.Desc {
	return x.desc
}

func (x *simplePrometheusMetric) Write(d *dto.Metric) error {
	d.Label = x.m.Label
	d.Gauge = x.m.Gauge
	d.Counter = x.m.Counter
	d.Summary = x.m.Summary
	d.Untyped = x.m.Untyped
	d.Histogram = x.m.Histogram
	d.TimestampMs = x.m.TimestampMs

	return nil
}

var _ prometheus.Metric = (*simplePrometheusMetric)(nil)
