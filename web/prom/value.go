package prom

import (
	"github.com/gogo/protobuf/proto"
	dto "github.com/prometheus/client_model/go"
)

// Field represents a single field key and value.
type Field struct {
	Key        string
	TypedValue TypedValue
}

type dtoValue *dto.Metric

type TypedValue interface {
	Copy() TypedValue // DeepCopy

	doNotImplementThisInterface() dtoValue
}

type typedValue struct {
	inner *dto.Metric
}

func (t typedValue) doNotImplementThisInterface() dtoValue {
	return t.inner
}

func (t typedValue) Copy() TypedValue {
	return typedValue{proto.Clone(t.inner).(*dto.Metric)}
}

// Inner is just for internal use, DO NOT CALL IT
func (t typedValue) Inner() *dto.Metric {
	return t.inner
}

func NewGaugeValue(v float64) TypedValue {
	return typedValue{&dto.Metric{
		Gauge: &dto.Gauge{
			Value: proto.Float64(v),
		},
	}}
}
