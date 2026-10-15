// Package eventbus 实现事件总线引擎: 经 ereg 提供 Bus 能力.
package eventbus

import (
	"sync"

	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/event"
)

// Engine 是进程内事件总线, 实现 event.Bus.
type Engine struct {
	mu   sync.RWMutex
	next int
	subs map[event.Topic]map[int]func(any)
}

// New 创建事件总线引擎.
func New() *Engine { return &Engine{subs: map[event.Topic]map[int]func(any){}} }

// Name 实现 engine.Engine.
func (e *Engine) Name() string { return "eventbus" }

// Deps 实现 engine.Engine.
func (e *Engine) Deps() []string { return nil }

// Init 实现 engine.Engine.
func (e *Engine) Init(context.Context) error { return nil }

// Start 实现 engine.Engine.
func (e *Engine) Start(context.Context) error { return nil }

// Stop 清理订阅.
func (e *Engine) Stop(context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.subs = map[event.Topic]map[int]func(any){}
	return nil
}

// Emit 实现 event.Bus, 向当前订阅者派发载荷.
func (e *Engine) Emit(topic event.Topic, payload any) {
	e.mu.RLock()
	handlers := make([]func(any), 0, len(e.subs[topic]))
	for _, fn := range e.subs[topic] {
		handlers = append(handlers, fn)
	}
	e.mu.RUnlock()
	for _, fn := range handlers {
		fn(payload)
	}
}

// Subscribe 实现 event.Bus, 返回注销函数.
func (e *Engine) Subscribe(topic event.Topic, fn func(payload any)) func() {
	e.mu.Lock()
	e.next++
	id := e.next
	if e.subs[topic] == nil {
		e.subs[topic] = map[int]func(any){}
	}
	e.subs[topic][id] = fn
	e.mu.Unlock()
	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.subs[topic], id)
	}
}
