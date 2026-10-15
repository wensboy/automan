package bridge

import (
	stdctx "context"
	"sort"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	"github.com/wensboy/automan/backend/internal/kits/bridgekit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// handlerEntry 是一个入站处理器.
type handlerEntry struct {
	fn         domainbridge.Handler
	concurrent bool
}

// Handle 实现 domain/bridge.Receiver: 按 name 注册处理器 (同名覆盖).
func (e *Engine) Handle(name string, h domainbridge.Handler, opts ...domainbridge.HandleOption) {
	cfg := domainbridge.HandleConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	e.mu.Lock()
	e.handlers[name] = handlerEntry{fn: h, concurrent: cfg.Concurrent}
	e.mu.Unlock()
}

// Send 实现 domain/bridge.Sender: 业务负载编码 -> ipc 消息 -> peer.
// ID 为空时由调用方自行关联 (引擎不做配对, 流语义属业务层).
func (e *Engine) Send(ctx stdctx.Context, out domainbridge.Outgoing) error {
	msg, err := bridgekit.EncodeMessage(e.codec, out)
	if err != nil {
		return err
	}
	peer := e.currentPeer()
	if peer == nil {
		return errNotConnected
	}
	return peer.Write(ctx, msg)
}

// registerBuiltin 注册协议内置处理器: 对端就绪通知与日志上行
// (不注册的话会被当作未注册 name 丢弃, 且每个日志都会产生一条 dropped 噪声).
func (e *Engine) registerBuiltin() {
	e.Handle(domainbridge.NameNodeReady, func(_ stdctx.Context, p domainbridge.Packet) error {
		var ready domainbridge.NodeReady
		_ = p.Decode(&ready)
		e.logInfo("bridge: node ready",
			logkit.F("pid", ready.PID), logkit.F("sock", ready.Sock))
		return nil
	})
	e.Handle(domainbridge.NameNodeLog, func(_ stdctx.Context, p domainbridge.Packet) error {
		var rec domainbridge.NodeLog
		if err := p.Decode(&rec); err != nil {
			return err
		}
		level, ok := logkit.ParseLevel(rec.Level)
		if !ok {
			level = domainlog.LevelInfo
		}
		fields := make([]domainlog.Field, 0, len(rec.Fields)+1)
		fields = append(fields, logkit.F("from", "node"))
		keys := make([]string, 0, len(rec.Fields))
		for k := range rec.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys) // 保证日志字段顺序确定
		for _, k := range keys {
			fields = append(fields, logkit.F(k, rec.Fields[k]))
		}
		e.logger.Deliver(logkit.New(level, rec.Msg, fields...))
		return nil
	})
}

// dispatch 把一条入站消息交给注册的处理器; 未注册的 name 丢弃 (不自动应答).
func (e *Engine) dispatch(ctx stdctx.Context, raw domainbridge.Message) {
	packet, err := bridgekit.DecodeMessage(e.codec, raw)
	if err != nil {
		e.logWarn("bridge: message dropped",
			logkit.F("reason", "decode-failed"),
			logkit.F("id", raw.ID),
			logkit.F("err", err))
		return
	}
	e.mu.Lock()
	entry, ok := e.handlers[packet.Name]
	e.mu.Unlock()
	if !ok {
		e.logWarn("bridge: message dropped",
			logkit.F("reason", "unregistered-name"),
			logkit.F("id", raw.ID),
			logkit.F("name", packet.Name))
		return
	}
	run := func() {
		if err := entry.fn(ctx, packet); err != nil {
			e.logWarn("bridge: handler failed",
				logkit.F("name", packet.Name), logkit.F("err", err))
		}
	}
	if entry.concurrent {
		go run()
		return
	}
	run()
}
