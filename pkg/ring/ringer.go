package ring

type Ringer[T any] interface {
	Do(func(T))
	Next() Ringer[T]
	Prev() Ringer[T]
	GetValue() T
	SetValue(T)
	Reset()
	Len() int
}
