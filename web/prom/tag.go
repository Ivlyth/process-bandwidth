package prom

import (
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

// Tag is an alias to prometheus LabelPair
type Tag = dto.LabelPair

func NewTag(name, value string) *Tag {
	return &dto.LabelPair{
		Name:  proto.String(name),
		Value: proto.String(value),
	}
}
