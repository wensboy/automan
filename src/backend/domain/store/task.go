package store

import "context"

// Cmd 是任务指令名.
type Cmd string

// Keyed 由指令参数实现, 提供分区键 (同键串行, 保证顺序).
type Keyed interface {
	Key() string
}

// Task 是一条 IO 指令.
type Task struct {
	Cmd  Cmd
	Args any
}

// Handler 是指令的处理函数, 由引擎按 Cmd 分派.
type Handler func(ctx context.Context, args any) (any, error)
