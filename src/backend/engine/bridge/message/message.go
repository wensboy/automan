// Package message 在帧层之上提供结构化消息: 请求/响应关联、事件分发、超时与背压。
//
// 约束(见 docs/design/v23.md §3.3): 只认 Envelope 与 Handler, 不碰 socket。
package message

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	stdctx "context"

	"github.com/wensboy/automan/backend/engine/bridge/frame"
)

// Version 是协议版本(v3 起信封直接放在文本帧里, 不再套 base64 帧)。
const Version = 3

// Kind 是消息类型。
type Kind string

const (
	// KindRequest 需要响应(带 ID)。
	KindRequest Kind = "request"
	// KindResponse 是成功响应(ID 回填)。
	KindResponse Kind = "response"
	// KindError 是失败响应(ID 回填)。
	KindError Kind = "error"
	// KindEvent 是单向通知(无响应)。
	KindEvent Kind = "event"
)

// Envelope 是文本帧里的 JSON 内容。
type Envelope struct {
	Version int             `json:"v"`
	Kind    Kind            `json:"kind"`
	Name    string          `json:"name,omitempty"`
	ID      string          `json:"id,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Handler 处理"对端发来的请求", 返回的数据作为 response 的 data。
type Handler func(ctx stdctx.Context, env Envelope) (any, error)

// EventHandler 处理"对端发来的事件"。
type EventHandler func(env Envelope)

// Options 是一条 peer 的运行参数(每个 peer 独立; 缺省值来自配置层)。
type Options struct {
	// RequestTimeout 是 Request 在未显式指定时的超时。
	RequestTimeout time.Duration
	// SendQueue 是发送队列长度: 满了立即返回 ErrBusy(不阻塞业务)。
	SendQueue int
	// MaxInflight 是在途请求上限。
	MaxInflight int
	// WriteTimeout 是单条消息的写入超时。
	WriteTimeout time.Duration
	// PingInterval 与 PongTimeout 决定心跳(双向: 两侧都可 ping)。
	PingInterval time.Duration
	PongTimeout  time.Duration
	// OnState 在连接状态变化时回调(connected/disconnected)。
	OnState func(state string)
	// Prepare 在启动读循环**之前**执行: 供调用方一次性注册处理器, 避免漏掉对端立刻发来的首条消息。
	Prepare func(p *Peer)
}

func (o Options) withDefaults() Options {
	if o.RequestTimeout <= 0 {
		o.RequestTimeout = 30 * time.Second
	}
	if o.SendQueue <= 0 {
		o.SendQueue = 256
	}
	if o.MaxInflight <= 0 {
		o.MaxInflight = 1024
	}
	if o.WriteTimeout <= 0 {
		o.WriteTimeout = 10 * time.Second
	}
	if o.PingInterval <= 0 {
		o.PingInterval = 20 * time.Second
	}
	if o.PongTimeout <= 0 {
		o.PongTimeout = 10 * time.Second
	}
	return o
}

var (
	// ErrClosed 表示连接已关闭。
	ErrClosed = errors.New("bridge/message: closed")
	// ErrBusy 表示队列/在途请求已满(背压, 业务可降级)。
	ErrBusy = errors.New("bridge/message: busy")
	// ErrTimeout 表示等待响应超时。
	ErrTimeout = errors.New("bridge/message: timeout")
)

// RemoteError 是对端返回的 kind=error。
type RemoteError struct {
	Name    string
	Code    string
	Message string
}

func (e *RemoteError) Error() string {
	code := e.Code
	if code == "" {
		code = "error"
	}
	return fmt.Sprintf("bridge: remote %s failed (%s): %s", e.Name, code, e.Message)
}

type pending struct {
	ch chan Envelope
}

type wireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Peer 是一条双向消息通道(父进程与子进程两侧都用它)。
type Peer struct {
	conn    *frame.Conn
	options Options

	mu       sync.Mutex
	pending  map[string]*pending
	handlers map[string]Handler
	events   map[string]EventHandler
	inflight int
	closed   bool

	writeCh   chan Envelope
	done      chan struct{}
	closeOnce sync.Once // 关 socket(发 close 帧)
	doneOnce  sync.Once // 收尾(shutdown); 与 closeOnce 分开, 避免 sync.Once 重入死锁
	lastPong  atomicTime
}

// New 创建 peer 并启动读/写/心跳三个协程。
func New(conn *frame.Conn, options Options) *Peer {
	p := &Peer{
		conn:     conn,
		options:  options.withDefaults(),
		pending:  map[string]*pending{},
		handlers: map[string]Handler{},
		events:   map[string]EventHandler{},
		writeCh:  make(chan Envelope, options.withDefaults().SendQueue),
		done:     make(chan struct{}),
	}
	p.lastPong.set(time.Now())
	// 收到任意帧(含 ping/pong)都刷新存活: 否则空闲连接会在一个心跳周期后被误判超时
	conn.SetActivityHandler(func() { p.lastPong.set(time.Now()) })
	if p.options.Prepare != nil {
		p.options.Prepare(p)
	}
	go p.readLoop()
	go p.writeLoop()
	go p.heartbeat()
	if p.options.OnState != nil {
		p.options.OnState("connected")
	}
	return p
}

// Done 在连接终止时关闭。
func (p *Peer) Done() <-chan struct{} { return p.done }

// Handle 注册"对端请求"的处理器(同名覆盖)。
func (p *Peer) Handle(name string, handler Handler) {
	p.mu.Lock()
	p.handlers[name] = handler
	p.mu.Unlock()
}

// OnEvent 注册事件订阅。
func (p *Peer) OnEvent(name string, handler EventHandler) {
	p.mu.Lock()
	p.events[name] = handler
	p.mu.Unlock()
}

// Notify 发送一条事件(无响应)。
func (p *Peer) Notify(ctx stdctx.Context, name string, data any) error {
	return p.enqueue(ctx, Envelope{Kind: KindEvent, Name: name}, data)
}

// Request 发送一条请求并等待响应。
func (p *Peer) Request(ctx stdctx.Context, name string, data any, timeout time.Duration) (json.RawMessage, error) {
	if timeout <= 0 {
		timeout = p.options.RequestTimeout
	}
	id := newID()

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrClosed
	}
	if p.inflight >= p.options.MaxInflight {
		p.mu.Unlock()
		return nil, ErrBusy
	}
	waiter := &pending{ch: make(chan Envelope, 1)}
	p.pending[id] = waiter
	p.inflight++
	p.mu.Unlock()

	defer p.release(id)

	waitCtx, cancel := stdctx.WithTimeout(ctx, timeout)
	defer cancel()

	if err := p.enqueue(ctx, Envelope{Kind: KindRequest, Name: name, ID: id}, data); err != nil {
		return nil, err
	}

	select {
	case env := <-waiter.ch:
		if env.Kind == KindError {
			return nil, decodeRemoteError(name, env.Data)
		}
		return env.Data, nil
	case <-waitCtx.Done():
		return nil, ErrTimeout
	case <-p.done:
		return nil, ErrClosed
	}
}

// Respond 处理完对端请求后回写响应(handler 由 readLoop 调用, 这里仅供扩展使用)。
func (p *Peer) Close(ctx stdctx.Context, reason string) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel stdctx.CancelFunc
		ctx, cancel = stdctx.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}
	p.closeOnce.Do(func() {
		_ = p.conn.Close(ctx, frame.CloseNormal, reason)
	})
	p.shutdown(ErrClosed)
}

func (p *Peer) release(id string) {
	p.mu.Lock()
	if _, ok := p.pending[id]; ok {
		delete(p.pending, id)
		p.inflight--
	}
	p.mu.Unlock()
}

func (p *Peer) shutdown(cause error) {
	p.doneOnce.Do(func() {
		close(p.done)
		p.mu.Lock()
		p.closed = true
		waiters := p.pending
		p.pending = map[string]*pending{}
		p.inflight = 0
		p.mu.Unlock()
		for _, waiter := range waiters {
			select {
			case waiter.ch <- Envelope{Kind: KindError, Data: mustJSON(wireError{Code: "closed", Message: cause.Error()})}:
			default:
			}
		}
		if p.options.OnState != nil {
			p.options.OnState("disconnected")
		}
	})
}

func (p *Peer) enqueue(ctx stdctx.Context, env Envelope, data any) error {
	env.Version = Version
	if data != nil {
		raw, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("bridge/message: encode data: %w", err)
		}
		env.Data = raw
	}
	select {
	case p.writeCh <- env:
		return nil
	case <-p.done:
		return ErrClosed
	default:
		return ErrBusy
	}
}

func (p *Peer) writeLoop() {
	for {
		select {
		case <-p.done:
			return
		case env := <-p.writeCh:
			payload, err := json.Marshal(env)
			if err != nil {
				continue
			}
			ctx, cancel := stdctx.WithTimeout(stdctx.Background(), p.options.WriteTimeout)
			err = p.conn.WriteMessage(ctx, frame.OpText, payload)
			cancel()
			if err != nil {
				if errors.Is(err, frame.ErrClosed) || errors.Is(err, frame.ErrProtocol) {
					p.shutdown(err)
				}
				return
			}
		}
	}
}

func (p *Peer) readLoop() {
	for {
		op, payload, err := p.conn.ReadMessage(stdctx.Background())
		if err != nil {
			p.shutdown(err)
			return
		}
		if op != frame.OpText {
			continue
		}
		var env Envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			p.shutdown(fmt.Errorf("bridge/message: bad envelope: %w", err))
			return
		}
		p.dispatch(env)
	}
}

func (p *Peer) dispatch(env Envelope) {
	switch env.Kind {
	case KindResponse, KindError:
		p.mu.Lock()
		waiter, ok := p.pending[env.ID]
		if ok {
			delete(p.pending, env.ID)
			p.inflight--
		}
		p.mu.Unlock()
		if ok {
			select {
			case waiter.ch <- env:
			default:
			}
		}
	case KindEvent:
		p.mu.Lock()
		eventHandler := p.events[env.Name]
		requestHandler := p.handlers[env.Name]
		p.mu.Unlock()
		if eventHandler != nil {
			eventHandler(env)
			return
		}
		// 事件也可由"按 name 注册的处理器"消费(引擎侧统一用 Handle 注册, 不必区分 event/request)
		if requestHandler != nil {
			go func() { _, _ = requestHandler(stdctx.Background(), env) }()
		}
	case KindRequest:
		p.mu.Lock()
		handler := p.handlers[env.Name]
		p.mu.Unlock()
		if handler == nil {
			_ = p.enqueue(stdctx.Background(), Envelope{
				Kind: KindError, Name: env.Name, ID: env.ID,
				Data: mustJSON(wireError{Code: "unregistered", Message: "no handler for " + env.Name}),
			}, nil)
			return
		}
		go func() {
			result, err := handler(stdctx.Background(), env)
			if err != nil {
				_ = p.enqueue(stdctx.Background(), Envelope{
					Kind: KindError, Name: env.Name, ID: env.ID,
					Data: mustJSON(wireError{Code: "handler_error", Message: err.Error()}),
				}, nil)
				return
			}
			_ = p.enqueue(stdctx.Background(), Envelope{
				Kind: KindResponse, Name: env.Name, ID: env.ID,
			}, result)
		}()
	}
}

func (p *Peer) heartbeat() {
	ticker := time.NewTicker(p.options.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			if time.Since(p.lastPong.get()) > p.options.PingInterval+p.options.PongTimeout {
				p.shutdown(ErrTimeout)
				return
			}
			ctx, cancel := stdctx.WithTimeout(stdctx.Background(), p.options.WriteTimeout)
			_ = p.conn.Ping(ctx, nil)
			cancel()
		}
	}
}

// OnPong 供 frame.Options 回调(心跳存活时间刷新)。
func (p *Peer) OnPong() { p.lastPong.set(time.Now()) }

func decodeRemoteError(name string, data json.RawMessage) error {
	var wire wireError
	if err := json.Unmarshal(data, &wire); err != nil {
		return &RemoteError{Name: name, Message: string(data)}
	}
	return &RemoteError{Name: name, Code: wire.Code, Message: wire.Message}
}

func mustJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

// newID 生成 ULID 风格的请求 ID(时间有序 + 随机后缀), 便于排查时序。
func newID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%013x%s", time.Now().UnixMilli(), hex.EncodeToString(buf))
}

// atomicTime 是轻量的原子时间封装(避免引入额外依赖)。
type atomicTime struct {
	mu  sync.RWMutex
	val time.Time
}

func (a *atomicTime) set(t time.Time) {
	a.mu.Lock()
	a.val = t
	a.mu.Unlock()
}

func (a *atomicTime) get() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.val
}
