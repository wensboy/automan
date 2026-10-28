// Package store 实现统一 IO 引擎: 复用 dispatch 指令核心, 只贡献 fs.* handler.
package store

import (
	stdctx "context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/context"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/dispatch"
	"github.com/wensboy/automan/backend/internal/fallback"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/paths"
)

// 缺省参数, 可经 data/conf/store.json 覆盖.
const (
	// defaultRoot 为 automan 数据目录本身 (各引擎的相对路径都落在此目录下).
	defaultRoot      = "."
	defaultShards    = 4
	defaultInflight  = 8
	defaultQueue     = 256
	defaultTimeoutMs = 10000
	defaultMaxSize   = 64 << 20
)

// 错误哨兵(供 handler 与调用方判定).

// Engine 是指令式 IO 引擎, 实现 domain/store 的 Store/Registrar/Statser.
type Engine struct {
	queue *dispatch.Queue

	root    string
	maxSize int64
}

// Option 设置 store 引擎的必要字段.
type Option func(*Engine)

// WithRoot 覆盖存储根 (缺省按平台约定解析).
func WithRoot(root string) Option { return func(e *Engine) { e.root = root } }

// New 创建 store 引擎; 执行器先按缺省参数就绪, Init 时再按配置重建.
func New(opts ...Option) *Engine {
	e := &Engine{queue: newQueue(config.Store{}), maxSize: defaultMaxSize}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Name 实现 engine.Engine.
func (e *Engine) Name() string { return "store" }

// Deps 实现 engine.Engine.
func (e *Engine) Deps() []string { return []string{"config", "eventbus"} }

// Init 解析配置参数并注册内置指令.
func (e *Engine) Init(ctx context.Context) error {
	provider, ok := enginekit.Of[config.Provider](ctx, "config")
	if !ok {
		return errConfigUnavailable
	}
	params, ok := configkit.Get(provider, config.StoreKey)
	if !ok {
		return fmt.Errorf("store: config section %q missing", config.StoreKey.Name())
	}
	if err := e.apply(params); err != nil {
		return err
	}
	return e.registerFS()
}

// Start 创建存储根并启动 worker.
func (e *Engine) Start(context.Context) error {
	if err := os.MkdirAll(e.root, 0o755); err != nil {
		return fmt.Errorf("store: create root %s: %w", e.root, err)
	}
	e.queue.Seal()
	e.queue.Launch()
	return nil
}

// Stop 停止接收并排空队列.
func (e *Engine) Stop(ctx context.Context) error { return e.queue.Close(ctx) }

// Register 实现 domain/store.Registrar, 仅允许 Start 之前注册.
func (e *Engine) Register(cmd domainstore.Cmd, h domainstore.Handler) error {
	return e.queue.Register(cmd, h)
}

// Submit 实现 domain/store.Store: 异步入队并立即返回结果句柄.
func (e *Engine) Submit(t domainstore.Task) <-chan domainstore.Result {
	return e.queue.Submit(t)
}

// Do 实现 domain/store.Store: 投递并阻塞等待结果.
func (e *Engine) Do(ctx stdctx.Context, t domainstore.Task) (any, error) {
	return e.queue.Do(ctx, t)
}

// Stats 实现 domain/store.Statser.
func (e *Engine) Stats() domainstore.Stats { return e.queue.Stats() }

// Root 实现 domain/store.Rooter: 暴露存储根 (绝对路径), 供 uds 等需要真实路径的引擎使用.
func (e *Engine) Root() string { return e.root }

// apply 解析存储根并把参数转换为 dispatch 配置.
func (e *Engine) apply(params config.Store) error {
	if e.root == "" {
		root, err := paths.Resolve(fallback.String(params.Root, defaultRoot))
		if err != nil {
			return err
		}
		e.root = root
	}
	e.maxSize = int64(fallback.Int(params.MaxSize, defaultMaxSize))
	e.queue = newQueue(params)
	return nil
}

// newQueue 按配置构造执行器, 缺省/非法取值退化为内置缺省.
func newQueue(params config.Store) *dispatch.Queue {
	return dispatch.New(dispatch.Config{
		Shards:   fallback.Int(params.Shards, defaultShards),
		Inflight: fallback.Int(params.Inflight, defaultInflight),
		Queue:    fallback.Int(params.Queue, defaultQueue),
		Timeout:  time.Duration(fallback.Int(params.TimeoutMs, defaultTimeoutMs)) * time.Millisecond,
	})
}

var errConfigUnavailable = errors.New(`store: engine "config" unavailable`)
