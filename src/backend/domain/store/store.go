// Package store 定义统一 IO 入口的领域契约.
// 上层只提交指令 (Task), 不感知队列/worker/分区/限流; 便捷调用见 internal/kits/storekit.
package store

import "context"

// Result 是一次任务的执行结果.
type Result struct {
	Value any
	Err   error
}

// Store 是调度入口: 接口极小, 能力经指令扩展.
type Store interface {
	// Submit 异步入队并返回结果句柄, 不阻塞调用方后续操作;
	// 通道收到恰好一条结果后关闭.
	Submit(task Task) <-chan Result
	Do(ctx context.Context, task Task) (any, error)
}

// Registrar 是扩展点: 注册指令的处理函数.
type Registrar interface {
	Register(cmd Cmd, h Handler) error
}

// Statser 暴露只读的队列与执行统计.
type Statser interface {
	Stats() Stats
}

// Stats 是 store 运行指标快照.
type Stats struct {
	Shards    int
	Depth     int
	Inflight  int
	Submitted uint64
	Completed uint64
	Failed    uint64
	Dropped   uint64
}
