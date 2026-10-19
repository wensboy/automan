package log

// Out key: 每个 out 的唯一名字, 统一登记在此 —— 新增 out 时先定义 key, 引擎与调用方都引用常量。
const (
	OutConsole = "console"
	OutDisk    = "disk"
	OutStream  = "stream"
	// OutNotify 是运行时提示的出口(内存环形, 不落盘).
	OutNotify = "notify"
)

// Out 是日志出口: 自带唯一 key; 过滤/格式化/批量/缓冲策略由 out 自持, 引擎不做任何过滤.
type Out interface {
	// Name 返回 out key (见 Out* 常量); 用于注册查重与定向投递匹配.
	Name() string
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

// Registrar 是扩展点: 注册 out (key 取自 Out.Name()).
type Registrar interface {
	Register(out Out) error
}

// Statser 暴露只读统计.
type Statser interface {
	Stats() Stats
}

// Streamer 由带"流式 out"的 logger 实现: 暴露二级总线, 供外部消费者订阅批量记录.
// 订阅者由冲刷 goroutine 同步调用, 必须快速返回; 单个订阅者异常会被隔离.
type Streamer interface {
	SubscribeStream(fn func([]Record)) (func(), error)
}

// Stats 是 log 引擎指标快照.
type Stats struct {
	Delivered uint64
	Dropped   uint64
	Flushed   uint64
	Buffered  int
	Outs      int
}
