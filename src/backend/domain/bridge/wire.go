package bridge

import (
	stdctx "context"
	"errors"
)

// 帧层通用契约(实现见 engine/bridge/frame)。
//
// 约束(见 docs/design/v23.md §3.2): 只认"消息边界 + 控制帧", 不认识业务 kind/name。
var (
	// ErrProtocol 表示帧格式/握手非法。
	ErrProtocol = errors.New("bridge: protocol error")
	// ErrTooLarge 表示消息超过上限。
	ErrTooLarge = errors.New("bridge: message too large")
)

// Opcode 是 WebSocket 帧类型。
type Opcode byte

const (
	// OpContinuation 是分片续帧。
	OpContinuation Opcode = 0x0
	// OpText 是文本帧(默认业务帧)。
	OpText Opcode = 0x1
	// OpBinary 是二进制帧(本期不用, 预留)。
	OpBinary Opcode = 0x2
	// OpClose 是关闭帧。
	OpClose Opcode = 0x8
	// OpPing 是心跳请求。
	OpPing Opcode = 0x9
	// OpPong 是心跳回应。
	OpPong Opcode = 0xA
)

// CloseCode 是关闭码(RFC6455 §7.4.1 常用值)。
type CloseCode uint16

const (
	// CloseNormal 是正常关闭。
	CloseNormal CloseCode = 1000
	// CloseGoingAway 是端点离开。
	CloseGoingAway CloseCode = 1001
	// CloseProtocolError 是协议错误。
	CloseProtocolError CloseCode = 1002
	// CloseTooLarge 是消息过大。
	CloseTooLarge CloseCode = 1009
)

// FrameConn 是"能把消息边界与控制帧处理好"的连接。
type FrameConn interface {
	// ReadMessage 读取一条完整消息(自动重组分片)。
	ReadMessage(ctx stdctx.Context) (Opcode, []byte, error)
	// WriteMessage 发送一条完整消息。
	WriteMessage(ctx stdctx.Context, op Opcode, payload []byte) error
	// Ping 发送心跳。
	Ping(ctx stdctx.Context, payload []byte) error
	// Close 执行关闭握手。
	Close(ctx stdctx.Context, code CloseCode, reason string) error
	// Done 在连接终止时关闭。
	Done() <-chan struct{}
}
