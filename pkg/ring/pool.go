package ring

import (
	"github.com/Ivlyth/process-bandwidth/pkg/profile"
	"sync"
	"time"
)

type Pool[T any] struct {
	pool       sync.Pool
	GetCounter profile.Counter
	PutCounter profile.Counter
}

func (rp *Pool[T]) Get() Ringer[T] {
	now := time.Now()
	v := rp.pool.Get().(Ringer[T])
	rp.GetCounter.Inc(time.Now().Sub(now))
	return v
}

func (rp *Pool[T]) Put(r Ringer[T]) {
	if r == nil {
		return
	}
	now := time.Now()
	r.Reset()
	rp.pool.Put(r)
	rp.PutCounter.Inc(time.Now().Sub(now))
}

func NewArrayRingPool[T any](ringSize int) Pool[T] {
	return Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return NewArrayRing[T](ringSize)
			},
		},
	}
}

func NewLinkRingPool[T any](ringSize int) Pool[T] {
	return Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return NewLinkRing[T](ringSize)
			},
		},
	}
}

var _ = NewArrayRingPool[any](1)
var _ = NewLinkRingPool[any](1)
