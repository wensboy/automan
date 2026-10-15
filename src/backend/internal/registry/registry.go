// Package registry 提供 Registry 默认实现: 依赖拓扑加载, 失败逆序回滚.
package registry

import (
	"fmt"
	"strings"

	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/engine"
)

// Registry 是 engine.Registry 的默认实现.
type Registry struct {
	engines []engine.Engine
	index   map[string]engine.Engine
	started []engine.Engine
}

// New 创建注册表.
func New() *Registry { return &Registry{index: map[string]engine.Engine{}} }

// Register 注册引擎, 校验重名/自依赖/重复依赖.
func (r *Registry) Register(e engine.Engine) error {
	name := e.Name()
	if _, dup := r.index[name]; dup {
		return fmt.Errorf("registry: duplicate engine %q", name)
	}
	seen := map[string]bool{}
	for _, d := range e.Deps() {
		if d == name {
			return fmt.Errorf("registry: engine %q depends on itself", name)
		}
		if seen[d] {
			return fmt.Errorf("registry: engine %q lists duplicate dependency %q", name, d)
		}
		seen[d] = true
	}
	r.index[name] = e
	r.engines = append(r.engines, e)
	return nil
}

// Engine 按名称返回引擎.
func (r *Registry) Engine(name string) (engine.Engine, bool) {
	e, ok := r.index[name]
	return e, ok
}

// Load 校验依赖并按其拓扑顺序加载, 任一失败则逆序回滚.
func (r *Registry) Load(ctx context.Context) error {
	for _, e := range r.engines {
		for _, d := range e.Deps() {
			if _, ok := r.index[d]; !ok {
				return fmt.Errorf("registry: engine %q depends on unknown engine %q", e.Name(), d)
			}
		}
	}
	order, err := r.order()
	if err != nil {
		return err
	}
	for _, e := range order {
		if err := e.Init(ctx); err != nil {
			_ = r.Unload(ctx)
			return fmt.Errorf("registry: engine %s init: %w", e.Name(), err)
		}
		if err := e.Start(ctx); err != nil {
			_ = r.Unload(ctx)
			return fmt.Errorf("registry: engine %s start: %w", e.Name(), err)
		}
		r.started = append(r.started, e)
	}
	return nil
}

// Unload 逆序停机已加载的引擎.
func (r *Registry) Unload(ctx context.Context) error {
	var first error
	for i := len(r.started) - 1; i >= 0; i-- {
		if err := r.started[i].Stop(ctx); err != nil && first == nil {
			first = err
		}
	}
	r.started = nil
	return first
}

func (r *Registry) order() ([]engine.Engine, error) {
	indeg := map[string]int{}
	dependents := map[string][]string{}
	for _, e := range r.engines {
		indeg[e.Name()] = len(e.Deps())
		for _, d := range e.Deps() {
			dependents[d] = append(dependents[d], e.Name())
		}
	}
	ready := []string{}
	for _, e := range r.engines {
		if indeg[e.Name()] == 0 {
			ready = append(ready, e.Name())
		}
	}
	order := make([]engine.Engine, 0, len(r.engines))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		order = append(order, r.index[name])
		for _, dep := range dependents[name] {
			indeg[dep]--
			if indeg[dep] == 0 {
				ready = append(ready, dep)
			}
		}
	}
	if len(order) != len(r.engines) {
		var pending []string
		for _, e := range r.engines {
			if indeg[e.Name()] > 0 {
				pending = append(pending, e.Name())
			}
		}
		return nil, fmt.Errorf("registry: cyclic dependency among %s", strings.Join(pending, ", "))
	}
	return order, nil
}
