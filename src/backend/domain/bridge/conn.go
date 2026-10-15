package bridge

import "context"

// Peer 是一条已建立的 ipc 连接 (近似 socket).
// 连接创建/释放的时机由引擎决定, peer 只负责收发与传输控制.
type Peer interface {
	// Read 读取下一条消息 (阻塞直到收到完整帧或出错).
	Read(ctx context.Context) (Message, error)
	// Write 发送一条消息 (并发安全, 顺序保持).
	Write(ctx context.Context, msg Message) error
	Close() error
	Local() string
	Remote() string
}
