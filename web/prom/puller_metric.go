package prom

import (
	"fmt"
	"hash/fnv"
	"time"
)

type metric struct {
	name   string
	tags   []*Tag
	fields []*Field
	tm     time.Time
}

func NewMetric(name string, tags []*Tag, fields map[string]TypedValue, tm time.Time) Metric {
	m := &metric{
		name:   name,
		tags:   tags,
		fields: nil,
		tm:     tm,
	}

	if len(fields) > 0 {
		m.fields = make([]*Field, 0, len(fields))
		for k, v := range fields {
			m.fields = append(m.fields, &Field{
				Key:        k,
				TypedValue: v,
			})
		}
	}

	return m
}

// FromMetric returns a deep copy of the metric with any tracking information
// removed.
func FromMetric(other Metric) Metric {
	m := &metric{
		name:   other.Name(),
		tags:   make([]*Tag, len(other.TagList())),
		fields: make([]*Field, len(other.FieldList())),
		tm:     other.Time(),
	}

	for i, tag := range other.TagList() {
		m.tags[i] = &Tag{Name: tag.Name, Value: tag.Value}
	}

	for i, field := range other.FieldList() {
		m.fields[i] = &Field{Key: field.Key, TypedValue: field.TypedValue.Copy()}
	}
	return m
}

func (m *metric) String() string {
	return fmt.Sprintf("%s %v %v %d", m.name, m.TagList(), m.FieldList(), m.tm.UnixNano())
}

func (m *metric) Name() string {
	return m.name
}

func (m *metric) TagList() []*Tag {
	return m.tags
}

func (m *metric) FieldList() []*Field {
	return m.fields
}

func (m *metric) Time() time.Time {
	return m.tm
}

func (m *metric) SetName(name string) {
	m.name = name
}

func (m *metric) AddTag(key, value string) {
	for i, tag := range m.tags {
		if key > *tag.Name {
			continue
		}

		if key == *tag.Name {
			tag.Value = &value
			return
		}

		m.tags = append(m.tags, nil)
		copy(m.tags[i+1:], m.tags[i:])
		m.tags[i] = &Tag{Name: &key, Value: &value}
		return
	}

	m.tags = append(m.tags, &Tag{Name: &key, Value: &value})
}

func (m *metric) HasTag(key string) bool {
	for _, tag := range m.tags {
		if *tag.Name == key {
			return true
		}
	}
	return false
}

func (m *metric) GetTag(key string) (string, bool) {
	for _, tag := range m.tags {
		if *tag.Name == key {
			return *tag.Value, true
		}
	}
	return "", false
}

func (m *metric) RemoveTag(key string) {
	for i, tag := range m.tags {
		if *tag.Name == key {
			copy(m.tags[i:], m.tags[i+1:])
			m.tags[len(m.tags)-1] = nil
			m.tags = m.tags[:len(m.tags)-1]
			return
		}
	}
}

func (m *metric) AddField(field *Field) {
	for i, currentField := range m.fields {
		if field.Key == currentField.Key {
			m.fields[i] = field
			return
		}
	}
	m.fields = append(m.fields, field)
}

func (m *metric) HasField(key string) bool {
	for _, field := range m.fields {
		if field.Key == key {
			return true
		}
	}
	return false
}

func (m *metric) GetField(key string) (TypedValue, bool) {
	for _, field := range m.fields {
		if field.Key == key {
			return field.TypedValue, true
		}
	}
	return nil, false
}

func (m *metric) RemoveField(key string) {
	for i, field := range m.fields {
		if field.Key == key {
			copy(m.fields[i:], m.fields[i+1:])
			m.fields[len(m.fields)-1] = nil
			m.fields = m.fields[:len(m.fields)-1]
			return
		}
	}
}

func (m *metric) SetTime(t time.Time) {
	m.tm = t
}

func (m *metric) Copy() Metric {
	m2 := &metric{
		name:   m.name,
		tags:   make([]*Tag, len(m.tags)),
		fields: make([]*Field, len(m.fields)),
		tm:     m.tm,
	}

	for i, tag := range m.tags {
		m2.tags[i] = &Tag{Name: tag.Name, Value: tag.Value}
	}

	for i, field := range m.fields {
		m2.fields[i] = &Field{Key: field.Key, TypedValue: field.TypedValue.Copy()}
	}
	return m2
}

func (m *metric) HashID() uint64 {
	h := fnv.New64a()
	h.Write([]byte(m.name))
	h.Write([]byte("\n"))
	for _, tag := range m.tags {
		h.Write([]byte(*tag.Name))
		h.Write([]byte("\n"))
		h.Write([]byte(*tag.Value))
		h.Write([]byte("\n"))
	}
	return h.Sum64()
}

func (m *metric) Accept() {
}

func (m *metric) Reject() {
}

func (m *metric) Drop() {
}

// Convert field to a supported type or nil if unconvertible
func convertField(v interface{}) interface{} {
	switch v := v.(type) {
	case float64:
		return v
	case int64:
		return v
	case string:
		return v
	case bool:
		return v
	case int:
		return int64(v)
	case uint:
		return uint64(v)
	case uint64:
		return v
	case []byte:
		return string(v)
	case int32:
		return int64(v)
	case int16:
		return int64(v)
	case int8:
		return int64(v)
	case uint32:
		return uint64(v)
	case uint16:
		return uint64(v)
	case uint8:
		return uint64(v)
	case float32:
		return float64(v)
	case *float64:
		if v != nil {
			return *v
		}
	case *int64:
		if v != nil {
			return *v
		}
	case *string:
		if v != nil {
			return *v
		}
	case *bool:
		if v != nil {
			return *v
		}
	case *int:
		if v != nil {
			return int64(*v)
		}
	case *uint:
		if v != nil {
			return uint64(*v)
		}
	case *uint64:
		if v != nil {
			return *v
		}
	case *[]byte:
		if v != nil {
			return string(*v)
		}
	case *int32:
		if v != nil {
			return int64(*v)
		}
	case *int16:
		if v != nil {
			return int64(*v)
		}
	case *int8:
		if v != nil {
			return int64(*v)
		}
	case *uint32:
		if v != nil {
			return uint64(*v)
		}
	case *uint16:
		if v != nil {
			return uint64(*v)
		}
	case *uint8:
		if v != nil {
			return uint64(*v)
		}
	case *float32:
		if v != nil {
			return float64(*v)
		}
	default:
		return nil
	}
	return nil
}
