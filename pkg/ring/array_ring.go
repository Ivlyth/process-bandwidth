package ring

type ArrayRingValue[T any] struct {
	valuable bool
	value    T
}

type ArrayRing[T any] struct {
	len int
	//array [30]any
	array []ArrayRingValue[T]
	i     int // current index
}

func (ar *ArrayRing[T]) Next() Ringer[T] {
	i := ar.i + 1
	if i == ar.len {
		i = 0
	}
	return &ArrayRing[T]{
		len:   ar.len,
		i:     i,
		array: ar.array,
	}
}

func (ar *ArrayRing[T]) Prev() Ringer[T] {
	i := ar.i - 1
	if i == -1 {
		i = ar.len - 1
	}
	return &ArrayRing[T]{
		len:   ar.len,
		i:     i,
		array: ar.array,
	}
}

func (ar *ArrayRing[T]) SetValue(t T) {
	ar.array[ar.i].valuable = true
	ar.array[ar.i].value = t
}

func (ar *ArrayRing[T]) GetValue() T {
	return ar.array[ar.i].value
}

func (ar *ArrayRing[T]) Do(f func(T)) {
	var start = ar.i
	for {
		if !ar.array[ar.i].valuable {
			// skip non-valuable value callback in the loop
			//f(zero[T]())
		} else {
			f(ar.array[ar.i].value)
		}

		ar.i++
		if ar.i == ar.len {
			ar.i = 0
		}
		if ar.i == start {
			break
		}
	}
}

func (ar *ArrayRing[T]) Reset() {
	for i, _ := range ar.array {
		ar.array[i].valuable = false
	}
}

func (ar *ArrayRing[T]) Len() int {
	return ar.len
}

func NewArrayRing[T any](n int) *ArrayRing[T] {
	return &ArrayRing[T]{
		len:   n,
		array: make([]ArrayRingValue[T], n),
		i:     0,
	}
}

var _ = NewArrayRing[any](1)
