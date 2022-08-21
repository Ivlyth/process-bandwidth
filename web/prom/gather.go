package prom

type Gather interface {
	// Gather 用于收集需要的数据信息
	Gather(Accumulator) error
}
