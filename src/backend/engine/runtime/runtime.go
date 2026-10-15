// Package runtime 是根上下文的唯一实现, 同时是依赖图的根引擎.
// 它只维护运行时骨架: 生命周期 + ereg(引擎表) + srvs(服务表).
package runtime

import (
	stdctx "context"
	"fmt"
	"sync"

	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/engine"
)

// Runtime 实现 context.Context 与 engine.Engine.
type Runtime struct {
	ctx    stdctx.Context
	cancel stdctx.CancelFunc
	mode   context.Mode
	ereg   engine.Registry
	srvs   sync.Map
}

type Option func(*Runtime)

func WithMode(mode context.Mode) Option {
	return func(r *Runtime) {
		r.mode = mode
	}
}

func WithRegistry(reg engine.Registry) Option {
	return func(r *Runtime) {
		r.ereg = reg
	}
}

// New 创建运行时.
func New(opts ...Option) *Runtime {
	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	rt := &Runtime{ctx: ctx, cancel: cancel}
	for _, fn := range opts {
		fn(rt)
	}
	return rt
}

// Name 实现 engine.Engine.
func (r *Runtime) Name() string { return "runtime" }

// Deps 实现 engine.Engine.
func (r *Runtime) Deps() []string { return nil }

// Init 实现 engine.Engine.
func (r *Runtime) Init(context.Context) error { return nil }

// Start 实现 engine.Engine, 驱动全部引擎的依赖拓扑加载.
func (r *Runtime) Start(ctx context.Context) error { return r.ereg.Load(ctx) }

// Stop 实现 engine.Engine, 逆序停机.
func (r *Runtime) Stop(ctx context.Context) error { return r.ereg.Unload(ctx) }

// Registry 返回引擎注册表; 引擎注册由组合根负责, runtime 只管理其生命周期.
func (r *Runtime) Registry() engine.Registry { return r.ereg }

// Done 实现 context.Context.
func (r *Runtime) Done() <-chan struct{} { return r.ctx.Done() }

// Err 实现 context.Context.
func (r *Runtime) Err() error { return r.ctx.Err() }

// Cancel 实现 context.Context.
func (r *Runtime) Cancel() { r.cancel() }

// Mode 实现 context.Context.
func (r *Runtime) Mode() context.Mode { return r.mode }

// Engine 实现 context.Context.
func (r *Runtime) Engine(name string) (any, bool) {
	e, ok := r.ereg.Engine(name)
	if !ok {
		return nil, false
	}
	return e, true
}

// Register 实现 context.Context, 写入 srvs.
func (r *Runtime) Register(key string, srv any) error {
	if _, loaded := r.srvs.LoadOrStore(key, srv); loaded {
		return fmt.Errorf("runtime: service %q already registered", key)
	}
	return nil
}

// Srv 实现 context.Context.
func (r *Runtime) Srv(key string) (any, bool) { return r.srvs.Load(key) }
