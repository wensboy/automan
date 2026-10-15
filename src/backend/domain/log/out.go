package log

// Out 是日志出口: 过滤/格式化/批量/缓冲策略由 out 自持, 引擎不做任何过滤.
type Out interface {
	Emit(records []Record) error
}

// Flusher 由需要缓冲的 out 实现.
type Flusher interface {
	Flush() error
}

// Buffer 由带缓冲的 out 实现, 暴露当前积压条数.
type Buffer interface {
	Buffered() int
}

// Registrar 是扩展点: 注册 out.
type Registrar interface {
	Register(name string, out Out) error
}

// Statser 暴露只读统计.
type Statser interface {
	Stats() Stats
}

// Stats 是 log 引擎指标快照.
type Stats struct {
	Delivered uint64
	Dropped   uint64
	Flushed   uint64
	Buffered  int
	Outs      int
}
