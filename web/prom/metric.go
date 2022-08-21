package prom

import "time"

// Metric is the type of data that is processed by Telegraf.  Input plugins,
// and to a lesser degree, Processor and Aggregator plugins create new Metrics
// and Output plugins write them.
type Metric interface {
	// Name is the primary identifier for the Metric and corresponds to the
	// measurement in the InfluxDB data model.
	Name() string

	// TagList returns the tags as a slice ordered by the tag key in lexical
	// bytewise ascending order.  The returned value should not be modified,
	// use the AddTag or RemoveTag methods instead.
	TagList() []*Tag

	// FieldList returns the fields as a slice in an undefined order.  The
	// returned value should not be modified, use the AddField or RemoveField
	// methods instead.
	FieldList() []*Field

	// Time returns the timestamp of the metric.
	Time() time.Time

	// SetName sets the metric name.
	SetName(name string)

	// GetTag returns the value of a tag and a boolean to indicate if it was set.
	GetTag(key string) (string, bool)

	// HasTag returns true if the tag is set on the Metric.
	HasTag(key string) bool

	// AddTag sets the tag on the Metric.  If the Metric already has the tag
	// set then the current value is replaced.
	AddTag(key, value string)

	// RemoveTag removes the tag if it is set.
	RemoveTag(key string)

	// GetField returns the value of a field and a boolean to indicate if it was set.
	GetField(key string) (TypedValue, bool)

	//// HasField returns true if the field is set on the Metric.
	//HasField(key string) bool
	//
	//// AddField sets the field on the Metric.  If the Metric already has the field
	//// set then the current value is replaced.
	//AddField(field *Field)
	//
	//// RemoveField removes the tag if it is set.
	//RemoveField(key string)

	// SetTime sets the timestamp of the Metric.
	SetTime(t time.Time)

	// HashID returns an unique identifier for the series.
	HashID() uint64

	// Copy returns a deep copy of the Metric.
	Copy() Metric

	// Accept marks the metric as processed successfully and written to an
	// output.
	Accept()

	// Reject marks the metric as processed unsuccessfully.
	Reject()

	// Drop marks the metric as processed successfully without being written
	// to any output.
	Drop()
}
