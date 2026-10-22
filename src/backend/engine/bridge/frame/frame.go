// Package frame 实现 RFC6455 子集: 握手升级、帧编解码(长度/掩码)、分片重组、控制帧。
//
// 约束(见 docs/design/v23.md §3.2): 只认"消息边界 + 控制帧", 不认识业务 kind/name。
package frame

import (
	"bufio"
	stdctx "context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strings"
	"sync"
	"time"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
	"github.com/wensboy/automan/backend/engine/bridge/transport"
)

// 帧类型、关闭码、错误与接口复用 domain/bridge 的通用契约。
type (
	Opcode    = domainbridge.Opcode
	CloseCode = domainbridge.CloseCode
)

const (
	OpContinuation     = domainbridge.OpContinuation
	OpText             = domainbridge.OpText
	OpBinary           = domainbridge.OpBinary
	OpClose            = domainbridge.OpClose
	OpPing             = domainbridge.OpPing
	OpPong             = domainbridge.OpPong
	CloseNormal        = domainbridge.CloseNormal
	CloseGoingAway     = domainbridge.CloseGoingAway
	CloseProtocolError = domainbridge.CloseProtocolError
	CloseTooLarge      = domainbridge.CloseTooLarge
)

var (
	ErrProtocol = domainbridge.ErrProtocol
	ErrTooLarge = domainbridge.ErrTooLarge
	ErrClosed   = domainbridge.ErrClosed
)

const (
	handshakePath = "/bridge"
	// closeGrace 是关闭帧往返的上限(v23 §7)。
	closeGrace = 2 * time.Second
	wsGUID     = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

// Options 是帧连接的运行参数。
type Options struct {
	MaxMessageBytes int64
	// OnPong 在收到 pong 时回调(心跳存活判定由上层用)。
	OnPong func()
}

// 编译期断言: 帧连接满足领域契约。
var _ domainbridge.FrameConn = (*Conn)(nil)

// Conn 是一条 WebSocket 连接(建立在 transport.Conn 之上)。
type Conn struct {
	raw     transport.Conn
	reader  *bufio.Reader
	options Options

	writeMu sync.Mutex
	// activity 在收到**任意**帧时回调(心跳/业务都算存活), 由上层注入。
	activityMu sync.RWMutex
	activity   func()

	done     chan struct{}
	closeOne sync.Once
}

// ServerUpgrade 执行**服务端**握手: 读取 HTTP 请求, 校验 Upgrade, 回 101。
func ServerUpgrade(ctx stdctx.Context, raw transport.Conn, timeout time.Duration, options Options) (*Conn, error) {
	if timeout > 0 {
		deadline := time.Now().Add(timeout)
		_ = raw.SetReadDeadline(deadline)
	}
	reader := bufio.NewReaderSize(raw, 4096)
	req, err := readRequest(reader)
	if err != nil {
		return nil, err
	}
	key := req.Get("Sec-WebSocket-Key")
	if key == "" || !strings.EqualFold(req.Get("Upgrade"), "websocket") {
		return nil, fmt.Errorf("%w: missing upgrade headers", ErrProtocol)
	}
	accept := acceptKey(key)
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	writeCtx := ctx
	if timeout > 0 {
		var cancel stdctx.CancelFunc
		writeCtx, cancel = stdctx.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := raw.Write(writeCtx, []byte(response)); err != nil {
		return nil, err
	}
	_ = raw.SetReadDeadline(time.Time{})
	return newConn(raw, reader, options), nil
}

// ClientUpgrade 执行**客户端**握手(GET + 校验 101), 供 Go 侧测试与将来的 Go 客户端使用。
func ClientUpgrade(ctx stdctx.Context, raw transport.Conn, timeout time.Duration, options Options) (*Conn, error) {
	key := base64.StdEncoding.EncodeToString([]byte("automan-bridge-key"))
	request := "GET " + handshakePath + " HTTP/1.1\r\n" +
		"Host: bridge\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	writeCtx := ctx
	if timeout > 0 {
		var cancel stdctx.CancelFunc
		writeCtx, cancel = stdctx.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := raw.Write(writeCtx, []byte(request)); err != nil {
		return nil, err
	}
	if timeout > 0 {
		_ = raw.SetReadDeadline(time.Now().Add(timeout))
	}
	reader := bufio.NewReaderSize(raw, 4096)
	status, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.Contains(status, "101") {
		return nil, fmt.Errorf("%w: unexpected status %q", ErrProtocol, strings.TrimSpace(status))
	}
	// 读到空行为止(丢掉响应头)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return nil, readErr
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	_ = raw.SetReadDeadline(time.Time{})
	return newConn(raw, reader, options), nil
}

func readRequest(reader *bufio.Reader) (textproto.MIMEHeader, error) {
	tp := textproto.NewReader(reader)
	line, err := tp.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("%w: read request line: %v", ErrProtocol, err)
	}
	if !strings.HasPrefix(line, "GET ") {
		return nil, fmt.Errorf("%w: unexpected request line %q", ErrProtocol, line)
	}
	header, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, fmt.Errorf("%w: read headers: %v", ErrProtocol, err)
	}
	return textproto.MIMEHeader(header), nil
}

func acceptKey(key string) string {
	sum := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func newConn(raw transport.Conn, reader *bufio.Reader, options Options) *Conn {
	if options.MaxMessageBytes <= 0 {
		options.MaxMessageBytes = 4 << 20
	}
	return &Conn{raw: raw, reader: reader, options: options, done: make(chan struct{})}
}

// SetActivityHandler 注入"收到任意帧"的回调(心跳存活判定用)。
func (c *Conn) SetActivityHandler(fn func()) {
	c.activityMu.Lock()
	c.activity = fn
	c.activityMu.Unlock()
}

func (c *Conn) touch() {
	c.activityMu.RLock()
	handler := c.activity
	c.activityMu.RUnlock()
	if handler != nil {
		handler()
	}
}

// Done 在连接终止时关闭(供上层事件化等待)。
func (c *Conn) Done() <-chan struct{} { return c.done }

// WriteMessage 发送一条完整消息(自动分片, 由调用方串行化语义)。
func (c *Conn) WriteMessage(ctx stdctx.Context, op Opcode, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.writeFrame(ctx, true, op, payload)
}

// Ping 发送心跳。
func (c *Conn) Ping(ctx stdctx.Context, payload []byte) error {
	return c.WriteMessage(ctx, OpPing, payload)
}

// Close 执行关闭握手: 发 close 帧 -> 等对端 close/超时 -> 关 fd。
func (c *Conn) Close(ctx stdctx.Context, code CloseCode, reason string) error {
	if code == 0 {
		code = CloseNormal
	}
	// 关闭是收尾动作: 调用方常传 Background, 这里补一个上限, 防止对端已死时永久阻塞
	if _, ok := ctx.Deadline(); !ok {
		var cancel stdctx.CancelFunc
		ctx, cancel = stdctx.WithTimeout(ctx, closeGrace)
		defer cancel()
	}
	payload := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(payload, uint16(code))
	copy(payload[2:], reason)
	err := c.WriteMessage(ctx, OpClose, payload)
	c.finish()
	return err
}

func (c *Conn) finish() {
	c.closeOne.Do(func() {
		close(c.done)
		_ = c.raw.Close()
	})
}

// ReadMessage 读取下一条**完整**消息(自动重组分片), 控制帧在内部处理。
func (c *Conn) ReadMessage(ctx stdctx.Context) (Opcode, []byte, error) {
	// 把 ctx 的到期时间同步到 socket 的读 deadline; bufio 的阻塞读据此返回超时
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.raw.SetReadDeadline(deadline)
	}
	var (
		messageOp   Opcode
		fragmented  bool
		accumulated []byte
	)
	for {
		fin, op, payload, err := c.readFrame()
		if err != nil {
			c.finish()
			if errors.Is(err, io.EOF) {
				return 0, nil, ErrClosed
			}
			return 0, nil, err
		}

		// 任何成功解析的帧都代表对端存活(仅靠文本帧会在空闲时误判超时)
		c.touch()

		switch op {
		case OpPing:
			// 自动回 pong(控制帧可插在分片消息中间)
			if err := c.WriteMessage(ctx, OpPong, payload); err != nil {
				return 0, nil, err
			}
			continue
		case OpPong:
			if c.options.OnPong != nil {
				c.options.OnPong()
			}
			continue
		case OpClose:
			_ = c.WriteMessage(ctx, OpClose, payload)
			c.finish()
			return OpClose, payload, ErrClosed
		case OpContinuation:
			if !fragmented {
				return 0, nil, fmt.Errorf("%w: unexpected continuation", ErrProtocol)
			}
		case OpText, OpBinary:
			if fragmented {
				return 0, nil, fmt.Errorf("%w: new message inside fragmentation", ErrProtocol)
			}
			messageOp = op
		default:
			return 0, nil, fmt.Errorf("%w: unknown opcode %x", ErrProtocol, op)
		}

		accumulated = append(accumulated, payload...)
		if int64(len(accumulated)) > c.options.MaxMessageBytes {
			return 0, nil, ErrTooLarge
		}
		fragmented = !fin
		if fin {
			return messageOp, accumulated, nil
		}
	}
}

func (c *Conn) readFrame() (bool, Opcode, []byte, error) {
	head := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, head); err != nil {
		return false, 0, nil, err
	}
	fin := head[0]&0x80 != 0
	op := Opcode(head[0] & 0x0F)
	masked := head[1]&0x80 != 0
	length := int64(head[1] & 0x7F)

	switch length {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, ext); err != nil {
			return false, 0, nil, err
		}
		length = int64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(c.reader, ext); err != nil {
			return false, 0, nil, err
		}
		length = int64(binary.BigEndian.Uint64(ext))
	}
	if length < 0 || length > c.options.MaxMessageBytes {
		return false, 0, nil, ErrTooLarge
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.reader, maskKey[:]); err != nil {
			return false, 0, nil, err
		}
	}

	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(c.reader, payload); err != nil {
			return false, 0, nil, err
		}
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return fin, op, payload, nil
}

// writeFrame 写一帧; 服务端不掩码(客户端必须掩码, 由 node 侧 ws 库负责)。
func (c *Conn) writeFrame(ctx stdctx.Context, fin bool, op Opcode, payload []byte) error {
	head := make([]byte, 0, 10)
	first := byte(op)
	if fin {
		first |= 0x80
	}
	head = append(head, first)

	length := len(payload)
	switch {
	case length <= 125:
		head = append(head, byte(length))
	case length <= 0xFFFF:
		head = append(head, 126, 0, 0)
		binary.BigEndian.PutUint16(head[2:], uint16(length))
	default:
		head = append(head, 127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(head[2:], uint64(length))
	}
	if err := c.raw.Write(ctx, append(head, payload...)); err != nil {
		return err
	}
	return nil
}
