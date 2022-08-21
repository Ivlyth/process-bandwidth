package ring

import "github.com/Ivlyth/process-bandwidth/pkg/generic"

type LinkRing[T any] struct {
	ring *Ring[T]
	len  int
}

func (lr *LinkRing[T]) Next() Ringer[T] {
	return &LinkRing[T]{
		ring: lr.ring.Next(),
		len:  lr.len,
	}
}

func (lr *LinkRing[T]) Prev() Ringer[T] {
	return &LinkRing[T]{
		ring: lr.ring.Prev(),
		len:  lr.len,
	}
}

func (lr *LinkRing[T]) SetValue(v T) {
	lr.ring.Value = v
}

func (lr *LinkRing[T]) GetValue() T {
	return lr.ring.Value
}

func (lr *LinkRing[T]) Do(f func(T)) {
	lr.ring.Do(f)
}

func (lr *LinkRing[T]) Reset() {
	if lr.ring != nil {
		lr.ring.Value = generic.ZeroValue[T]()
		for p := lr.ring.Next(); p != lr.ring; p = p.Next() {
			p.Value = generic.ZeroValue[T]()
		}
	}
}

func (lr *LinkRing[T]) Len() int {
	return lr.len
}

func NewLinkRing[T any](n int) *LinkRing[T] {
	return &LinkRing[T]{
		ring: NewRing[T](n),
		len:  n,
	}
}

var _ = NewLinkRing[any](1)
