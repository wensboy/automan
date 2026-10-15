// Package engine 定义引擎契约与注册表能力 (解析辅助见 internal/kits/enginekit).
package engine

import "github.com/wensboy/automan/backend/domain/context"

// Engine 是可注册、按依赖拓扑加载的组件.
type Engine interface {
	Name() string
	Deps() []string
	Init(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Registry 管理引擎的注册、解析与生命周期.
type Registry interface {
	Register(e Engine) error
	Engine(name string) (Engine, bool)
	Load(ctx context.Context) error
	Unload(ctx context.Context) error
}
