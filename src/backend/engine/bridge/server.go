package bridge

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"time"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
	"github.com/wensboy/automan/backend/domain/event"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	"github.com/wensboy/automan/backend/engine/bridge/frame"
	"github.com/wensboy/automan/backend/engine/bridge/message"
	"github.com/wensboy/automan/backend/engine/bridge/transport"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// 协议内置 name(与 node 侧逐字一致)。
const (
	nameHandshake = "bridge.handshake"
	nameNodeReady = "node.ready"
	nameNodeLog   = "node.log"
)

// sockPath 返回 socket 的绝对路径(配置里的相对路径落到数据根下)。
func (e *Engine) sockPath() string {
	if filepath.IsAbs(e.sock) {
		return e.sock
	}
	if e.rooter != nil {
		return filepath.Join(e.rooter.Root(), filepath.FromSlash(e.sock))
	}
	return filepath.FromSlash(e.sock)
}

// listen 创建 UDS 监听面。
func (e *Engine) listen() error {
	listener, err := transport.Listen(e.sockPath())
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.server = listener
	e.mu.Unlock()
	e.logInfo("bridge: listening",
		logkit.F("sock", listener.Addr()),
		logkit.F("codec", e.codecName),
		logkit.F("command", e.command))
	return nil
}

// rebind 重绑监听 (幂等): 关闭旧监听 -> 重新 Listen。
func (e *Engine) rebind() error {
	e.closeServer()
	return e.listen()
}

func (e *Engine) closeServer() {
	e.mu.Lock()
	listener := e.server
	e.server = nil
	e.mu.Unlock()
	if listener != nil {
		_ = listener.Close()
	}
}

// serve 接受子进程连接: 握手 -> 建 peer -> 注册处理器 -> 等连接结束。
func (e *Engine) serve(ctx stdctx.Context) {
	defer e.wg.Done()
	for {
		listener := e.currentServer()
		if listener == nil {
			return
		}
		conn, err := listener.Accept(ctx)
		if err != nil {
			if errors.Is(err, transport.ErrClosed) || ctx.Err() != nil {
				return
			}
			e.logWarn("bridge: accept failed", logkit.F("err", err))
			continue
		}

		framed, err := frame.ServerUpgrade(ctx, conn, e.handshakeTimeout, frame.Options{MaxMessageBytes: e.maxMessageBytes})
		if err != nil {
			e.logWarn("bridge: upgrade failed", logkit.F("err", err))
			_ = conn.Close()
			continue
		}

		peer := message.New(framed, message.Options{
			PingInterval: e.heartbeat,
			PongTimeout:  e.heartbeatGrace,
			OnState:      func(state string) { e.onPeerState(state) },
			// 在读循环启动前注册: 子进程连上后会立刻发握手请求, 不能漏
			Prepare: func(p *message.Peer) { e.registerPeerHandlers(p) },
		})
		e.setPeer(peer)

		<-peer.Done()
		if e.currentPeer() == peer {
			e.setPeer(nil)
		}
		e.logWarn("bridge: peer disconnected")
	}
}

func (e *Engine) onPeerState(state string) {
	if state == "connected" {
		e.logInfo("bridge: peer connected")
		e.emit(event.TopicBridgeConnected, nil)
		return
	}
	e.emit(event.TopicBridgeDisconnected, nil)
}

// registerPeerHandlers 把引擎的处理器表 + 协议内置处理挂到 peer 上。
func (e *Engine) registerPeerHandlers(peer *message.Peer) {
	peer.Handle(nameHandshake, func(_ stdctx.Context, env message.Envelope) (any, error) {
		return e.handleHandshake(env)
	})
	peer.Handle(nameNodeReady, func(_ stdctx.Context, env message.Envelope) (any, error) {
		var ready domainbridge.NodeReady
		if err := json.Unmarshal(env.Data, &ready); err == nil {
			e.logInfo("bridge: node ready", logkit.F("pid", ready.PID), logkit.F("sock", ready.Sock))
		}
		return nil, nil
	})
	peer.Handle(nameNodeLog, func(_ stdctx.Context, env message.Envelope) (any, error) {
		e.forwardNodeLog(env.Data)
		return nil, nil
	})

	e.mu.Lock()
	entries := make(map[string]handlerEntry, len(e.handlers))
	for name, entry := range e.handlers {
		entries[name] = entry
	}
	e.mu.Unlock()
	for name, entry := range entries {
		handler := entry
		peer.Handle(name, func(ctx stdctx.Context, env message.Envelope) (any, error) {
			packet := domainbridge.Packet{
				ID: env.ID, Kind: kindOf(env.Kind), Name: env.Name, Data: env.Data, Codec: e.codec,
			}
			if err := handler.fn(ctx, packet); err != nil {
				e.logWarn("bridge: handler failed", logkit.F("name", env.Name), logkit.F("err", err))
				return nil, err
			}
			return nil, nil
		})
	}
}

// handleHandshake 校验版本与 token, 返回 {daemon, codec, version}。
func (e *Engine) handleHandshake(env message.Envelope) (any, error) {
	var request domainbridge.Handshake
	if err := json.Unmarshal(env.Data, &request); err != nil {
		return nil, err
	}
	if request.Version != message.Version {
		return nil, errors.New("bridge: protocol version mismatch")
	}
	if e.token != "" && request.Token != e.token {
		return nil, errors.New("bridge: handshake token rejected")
	}
	return domainbridge.HandshakeResult{
		Daemon: e.daemon, Codec: e.codecName, Version: message.Version,
	}, nil
}

// Handle 实现 domain/bridge.Receiver: 按 name 注册处理器(同名覆盖), 并同步到当前 peer。
func (e *Engine) Handle(name string, h domainbridge.Handler, opts ...domainbridge.HandleOption) {
	cfg := domainbridge.HandleConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	e.mu.Lock()
	e.handlers[name] = handlerEntry{fn: h, concurrent: cfg.Concurrent}
	e.mu.Unlock()

	peer := e.currentPeer()
	if peer == nil {
		return
	}
	peer.Handle(name, func(ctx stdctx.Context, env message.Envelope) (any, error) {
		packet := domainbridge.Packet{ID: env.ID, Kind: kindOf(env.Kind), Name: env.Name, Data: env.Data, Codec: e.codec}
		if cfg.Concurrent {
			go func() { _ = h(ctx, packet) }()
			return nil, nil
		}
		return nil, h(ctx, packet)
	})
}

// Send 实现 domain/bridge.Sender: 按 kind 走事件或请求(默认事件, 语义与旧实现一致)。
func (e *Engine) Send(ctx stdctx.Context, out domainbridge.Outgoing) error {
	peer := e.currentPeer()
	if peer == nil {
		return errNotConnected
	}
	switch out.Kind {
	case domainbridge.KindEvent, "":
		return peer.Notify(ctx, out.Name, out.Data)
	case domainbridge.KindError:
		return peer.Notify(ctx, out.Name, map[string]any{"code": "error", "message": out.Data})
	default:
		// stream 语义: 仍是"发出不等响应"(响应对由业务按 name 自行关联, 保持既有调用不变)
		return peer.Notify(ctx, out.Name, out.Data)
	}
}

// 编译期断言: 引擎同时提供请求-响应能力。
var _ domainbridge.Requester = (*Engine)(nil)

// Request 是消息层的请求-响应能力(供引擎内部与后续业务直接使用)。
func (e *Engine) Request(ctx stdctx.Context, name string, data any, timeout time.Duration) (json.RawMessage, error) {
	peer := e.currentPeer()
	if peer == nil {
		return nil, errNotConnected
	}
	return peer.Request(ctx, name, data, timeout)
}

func (e *Engine) currentServer() *transport.Listener {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.server
}

func (e *Engine) currentPeer() *message.Peer {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.peer
}

func (e *Engine) setPeer(peer *message.Peer) {
	e.mu.Lock()
	e.peer = peer
	e.mu.Unlock()
}

// forwardNodeLog 把子进程上报的日志转成引擎日志(字段排序保证稳定)。
func (e *Engine) forwardNodeLog(data json.RawMessage) {
	var rec domainbridge.NodeLog
	if err := json.Unmarshal(data, &rec); err != nil {
		return
	}
	level, ok := logkit.ParseLevel(rec.Level)
	if !ok {
		level = domainlog.LevelInfo
	}
	fields := make([]domainlog.Field, 0, len(rec.Fields)+1)
	fields = append(fields, logkit.F("from", "node"))
	keys := make([]string, 0, len(rec.Fields))
	for key := range rec.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fields = append(fields, logkit.F(key, rec.Fields[key]))
	}
	e.logger.Deliver(logkit.New(level, rec.Msg, fields...))
}

func kindOf(kind message.Kind) domainbridge.Kind {
	switch kind {
	case message.KindRequest:
		return domainbridge.KindStream
	case message.KindError:
		return domainbridge.KindError
	default:
		return domainbridge.KindEvent
	}
}
