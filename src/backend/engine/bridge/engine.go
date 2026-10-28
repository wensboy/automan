// Package bridge 实现桥接引擎: peer 生命周期编排 + 业务处理 + 子进程编排.
// 传输/帧/消息分三层, 见 engine/bridge/{transport,frame,message}(docs/design/v23.md).
package bridge

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
	"github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/event"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/engine/bridge/message"
	"github.com/wensboy/automan/backend/engine/bridge/transport"
	"github.com/wensboy/automan/backend/internal/fallback"
	"github.com/wensboy/automan/backend/internal/kits/bridgekit"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// 内置缺省, 优先级: option > conf/bridge.<daemon> > 此处.
const (
	defaultDaemon         = ""
	defaultCodec          = "json"
	defaultHandshakeMs    = 5000
	defaultHeartbeatMs    = 15000
	defaultHeartbeatGrace = 5000
	defaultRestartMax     = 32
	stopGrace             = 3 * time.Second
	backoffBase           = 200 * time.Millisecond
	backoffMax            = 5 * time.Second
	// assetScript 是内嵌产物中作为入口的文件名; assetCommand 是拉起它的命令.
	assetScript  = "index.js"
	assetCommand = "node"
)

// Option 设置桥接引擎的必要字段 (创建实例时配置, 优先于配置文件).
type Option func(*Engine)

// WithDaemon 设置被桥接 daemon 名 (决定引擎名 bridge.<daemon> 与缺省 sock).
func WithDaemon(name string) Option { return func(e *Engine) { e.daemon = name } }

// WithSock 覆盖 sock 相对存储根的路径.
func WithSock(path string) Option { return func(e *Engine) { e.sock = path } }

// WithRunner 注入 daemon 对应的运行器工厂(为空表示附着模式: 不拉起子进程)。
func WithRunner(factory domainbridge.RunnerFactory) Option {
	return func(e *Engine) { e.factory = factory }
}

// WithRestart 设置重启策略与最大次数.
func WithRestart(policy string, max int) Option {
	return func(e *Engine) {
		e.restart = policy
		e.restartMax = max
	}
}

// handlerEntry 是一个入站处理器。
type handlerEntry struct {
	fn         domainbridge.Handler
	concurrent bool
}

// Engine 是桥接引擎: 编排 peer 生命周期与子进程, 并把业务消息交给注册的处理器.
type Engine struct {
	// 参数 (option)
	daemon           string
	sock             string
	codecName        string
	restart          string
	restartMax       int
	maxMessageBytes  int64
	handshakeTimeout time.Duration
	heartbeat        time.Duration
	heartbeatGrace   time.Duration
	token            string

	// 依赖
	store  domainstore.Store
	rooter domainstore.Rooter
	logger domainlog.Logger
	ebus   event.Bus

	// 业务
	codec    domainbridge.Codec
	handlers map[string]handlerEntry

	// 运行态
	mu      sync.Mutex
	wg      sync.WaitGroup
	runCtx  stdctx.Context
	cancel  stdctx.CancelFunc
	enabled bool
	server  *transport.Listener
	peer    *message.Peer
	runner  domainbridge.Runner
	factory domainbridge.RunnerFactory
	params  json.RawMessage
}

// New 创建桥接引擎.
func New(opts ...Option) *Engine {
	e := &Engine{daemon: defaultDaemon, handlers: map[string]handlerEntry{}}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Name 实现 engine.Engine.
func (e *Engine) Name() string { return "bridge." + e.daemon }

// Deps 实现 engine.Engine.
// settings/database 为声明依赖 (业务可调项与后续审计), 当前实现尚未使用.
func (e *Engine) Deps() []string {
	return []string{"config", "database", "eventbus", "log", "settings", "store"}
}

// Init 解析参数并装配 (不启动).
func (e *Engine) Init(ctx context.Context) error {
	provider, ok := enginekit.Of[config.Provider](ctx, "config")
	if !ok {
		return errConfigUnavailable
	}
	store, ok := enginekit.Of[domainstore.Store](ctx, "store")
	if !ok {
		return errStoreUnavailable
	}
	rooter, ok := enginekit.Of[domainstore.Rooter](ctx, "store")
	if !ok {
		return errRootUnavailable
	}
	logger, ok := enginekit.Of[domainlog.Logger](ctx, "log")
	if !ok {
		return errLogUnavailable
	}
	ebus, ok := enginekit.Of[event.Bus](ctx, "eventbus")
	if !ok {
		return errEventBusUnavailable
	}
	e.store, e.rooter, e.logger, e.ebus = store, rooter, logger, ebus

	params, _ := configkit.Get[config.BridgeParams](provider, config.BridgeKey)
	e.apply(params[e.daemon])

	codec, ok := bridgekit.Codec(e.codecName)
	if !ok {
		return fmt.Errorf("bridge: unknown payload codec %q", e.codecName)
	}
	e.codec = codec
	return nil
}

// Start 启动监听与 (可选) 子进程, 进入接受循环.
func (e *Engine) Start(context.Context) error {
	if !e.enabled {
		e.logInfo("bridge: disabled")
		return nil
	}
	runCtx, cancel := stdctx.WithCancel(stdctx.Background())
	e.mu.Lock()
	e.runCtx, e.cancel = runCtx, cancel
	e.mu.Unlock()

	if err := e.listen(); err != nil {
		return err
	}
	if err := e.startProcess(runCtx); err != nil {
		return err
	}
	e.wg.Add(1)
	go e.serve(runCtx)
	if e.factory != nil {
		e.wg.Add(1)
		go e.supervise(runCtx)
	}
	return nil
}

// Stop 停止接受与子进程, 关闭监听 (幂等).
func (e *Engine) Stop(context.Context) error {
	if !e.enabled {
		return nil
	}
	e.mu.Lock()
	cancel := e.cancel
	e.cancel = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	// 先终止子进程: supervise 阻塞在 Wait 上, 不先杀它会一直等下去.
	e.stopProcess(stdctx.Background())
	if peer := e.currentPeer(); peer != nil {
		peer.Close(stdctx.Background(), "engine stop")
	}
	// 有界等待内部协程 (子进程极端情况下可能杀不掉, 也不能把停机挂死).
	if !waitGroup(&e.wg, stopGrace) {
		e.logError("bridge: shutdown did not finish in time")
	}
	e.closeServer()
	e.logInfo("bridge: stopped", logkit.F("sock", e.sock))
	return nil
}

func (e *Engine) currentRunner() domainbridge.Runner {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runner
}

// apply 合并参数: option > 配置 > 内置缺省.
func (e *Engine) apply(cfg config.Bridge) {
	e.enabled = cfg.Enable == nil || *cfg.Enable
	e.sock = pick(e.sock, cfg.Sock, "bridge/"+fallback.String(e.daemon, "bridge")+".sock")
	e.codecName = pick(e.codecName, cfg.Codec, defaultCodec)
	// daemon 专属参数原样透传给运行器(运行器自行解析 command/env/services 等)
	e.params = marshalParams(cfg)
	e.token = pick(e.token, cfg.Token, "")
	e.restart = pick(e.restart, cfg.Restart, restartNone)
	e.restartMax = pickInt(e.restartMax, cfg.RestartMax, defaultRestartMax)
	e.maxMessageBytes = pickInt64(e.maxMessageBytes, cfg.MaxMessageBytes, 0)
	e.handshakeTimeout = pickMs(e.handshakeTimeout, cfg.HandshakeTimeoutMs, defaultHandshakeMs*time.Millisecond)
	e.heartbeat = pickMs(e.heartbeat, cfg.HeartbeatMs, defaultHeartbeatMs*time.Millisecond)
	e.heartbeatGrace = pickMs(e.heartbeatGrace, cfg.HeartbeatGraceMs, defaultHeartbeatGrace*time.Millisecond)
}

func pick(option, cfg, def string) string {
	return fallback.String(option, fallback.String(cfg, def))
}

func pickInt(option, cfg, def int) int {
	if option > 0 {
		return option
	}
	return fallback.Int(cfg, def)
}

func pickInt64(option, cfg, def int64) int64 {
	if option > 0 {
		return option
	}
	return fallback.Int64(cfg, def)
}

func pickMs(option time.Duration, cfgMs int, def time.Duration) time.Duration {
	if option > 0 {
		return option
	}
	if cfgMs > 0 {
		return time.Duration(cfgMs) * time.Millisecond
	}
	return def
}

func (e *Engine) logInfo(msg string, fields ...domainlog.Field) {
	e.logger.Deliver(logkit.New(domainlog.LevelInfo, msg, fields...))
}

func (e *Engine) logWarn(msg string, fields ...domainlog.Field) {
	e.logger.Deliver(logkit.New(domainlog.LevelWarn, msg, fields...))
}

func (e *Engine) logError(msg string, fields ...domainlog.Field) {
	e.logger.Deliver(logkit.New(domainlog.LevelError, msg, fields...))
}

func (e *Engine) emit(topic event.Topic, payload any) {
	e.ebus.Emit(topic, payload)
}

var (
	errConfigUnavailable   = errors.New(`bridge: engine "config" unavailable`)
	errStoreUnavailable    = errors.New(`bridge: engine "store" unavailable`)
	errRootUnavailable     = errors.New(`bridge: store root unavailable`)
	errLogUnavailable      = errors.New(`bridge: engine "log" unavailable`)
	errEventBusUnavailable = errors.New(`bridge: engine "eventbus" unavailable`)
	errNotConnected        = errors.New("bridge: not connected")
)
