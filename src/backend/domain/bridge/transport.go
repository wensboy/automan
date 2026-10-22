package bridge

import (
	stdctx "context"
	"errors"
	"time"
)

// 传输层通用契约(实现见 engine/bridge/transport)。
//
// 约束(见 docs/design/v23.md §3.1): 只认 []byte 与 deadline, 不认识 WebSocket, 也不认识 JSON。
var (
	// ErrClosed 表示连接/监听已关闭(本地原因)。
	ErrClosed = errors.New("bridge: closed")
	// ErrTimeout 表示读写超过 deadline 或 ctx 结束。
	ErrTimeout = errors.New("bridge: timeout")
	// ErrBackpressure 表示写缓冲满/写超时(上层应降级或拒绝, 而不是排队)。
	ErrBackpressure = errors.New("bridge: backpressure")
)

// Stream 是一条已建立的**字节流**连接。
type Stream interface {
	// Read 读字节(尊重已设置的 deadline), 供 bufio / io.ReadFull 这类标准库消费者使用。
	Read(p []byte) (int, error)
	// ReadContext 是按 ctx 设置 deadline 后读取的便捷形式。
	ReadContext(ctx stdctx.Context, p []byte) (int, error)
	// Write 写完整个 p; 写不动时返回 ErrBackpressure。
	Write(ctx stdctx.Context, p []byte) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	Close() error
	Local() string
	Remote() string
}

// Acceptor 是监听器(服务端侧)。
type Acceptor interface {
	Accept(ctx stdctx.Context) (Stream, error)
	Addr() string
	Close() error
}
