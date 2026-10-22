package bridge

import (
	"context"
	"encoding/json"
	"time"
)

// Kind 是消息类型 (业务层语义; 传输层只透传, 不解释).
type Kind string

const (
	// KindStream 代表 1:n / n:n 关系的消息 (流的唯一含义).
	KindStream Kind = "stream"
	// KindEvent 是单向通知.
	KindEvent Kind = "event"
	// KindError 是失败消息, Data 为原因.
	KindError Kind = "error"
)

// Envelope 是负载内的协议信封 (业务标准).
type Envelope struct {
	Kind Kind   `json:"kind"`
	Name string `json:"name,omitempty"`
	Data []byte `json:"data,omitempty"`
}

// Packet 是引擎交给业务层的一条消息.
type Packet struct {
	ID    string
	Kind  Kind
	Name  string
	Data  []byte
	Codec Codec // 供业务层解码 Data
}

// Decode 把业务负载解码为具体类型.
func (p Packet) Decode(into any) error { return p.Codec.Decode(p.Data, into) }

// Outgoing 是业务要发出的一条消息 (ID 为空时由引擎生成).
type Outgoing struct {
	ID   string
	Kind Kind
	Name string
	Data any
}

// Requester 是"请求-响应"能力(响应与请求按内部 ID 关联, 业务不需要自己配表)。
type Requester interface {
	// Request 发一条请求并等待响应; data 为响应负载(reason 失败时由 error 给出)。
	Request(ctx context.Context, name string, data any, timeout time.Duration) (json.RawMessage, error)
}

// Sender 是出站能力.
type Sender interface {
	Send(ctx context.Context, msg Outgoing) error
}

// Handler 处理一条入站消息 (返回 error 仅用于日志/观测).
type Handler func(ctx context.Context, p Packet) error

// Receiver 是入站能力: 按 name 注册处理器.
type Receiver interface {
	Handle(name string, h Handler, opts ...HandleOption)
}

// HandleConfig 是入站处理器的显式配置.
type HandleConfig struct {
	Concurrent bool // 该 name 的入站消息是否并行处理 (缺省串行)
}

// HandleOption 调整入站处理器的行为 (构造见 internal/kits/bridgekit).
type HandleOption func(*HandleConfig)

// 握手: 会话建立后、进入分派前的一次性自检 (不协商编码, codec 名仅作断言).
const (
	// NameHandshake 是握手消息名.
	NameHandshake = "bridge.handshake"
	// NameNodeReady 是对端(子进程)就绪通知名 (kind=event).
	NameNodeReady = "node.ready"
	// NameNodeLog 是对端日志上行名 (kind=event).
	NameNodeLog = "node.log"
	// ProtocolVersion 是当前协议版本 (帧与协议语义变更时递增).
	// v3: 信封直接放在文本帧里(不再套 base64 帧), 见 docs/design/v23.md
	ProtocolVersion = 3
)

// NodeReady 是就绪通知载荷.
type NodeReady struct {
	PID  int    `json:"pid"`
	Sock string `json:"sock"`
}

// NodeLog 是日志上行载荷.
type NodeLog struct {
	Level  string         `json:"level"`
	Msg    string         `json:"msg"`
	Fields map[string]any `json:"fields,omitempty"`
}

// Handshake 是握手请求载荷.
type Handshake struct {
	Daemon  string `json:"daemon"`
	Codec   string `json:"codec"`
	Version int    `json:"version"`
	Token   string `json:"token,omitempty"`
}

// HandshakeResult 是握手应答载荷.
type HandshakeResult struct {
	Daemon  string `json:"daemon"`
	Codec   string `json:"codec"`
	Version int    `json:"version"`
}
