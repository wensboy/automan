package log

import (
	"fmt"

	domainlog "github.com/wensboy/automan/backend/domain/log"
)

// Deliver 实现 domain/log.Logger 的扇出投递: 分发给所有 out 后立即返回, 不阻塞投递方.
// 引擎不做任何级别过滤, 过滤策略由各 out 自持.
func (e *Engine) Deliver(record domainlog.Record) {
	e.delivered.Add(1)
	e.mu.RLock()
	outs := e.outs
	e.mu.RUnlock()
	for _, out := range outs {
		_ = out.Emit([]domainlog.Record{record})
	}
}

// DeliverTo 实现 domain/log.Logger 的定向投递: 只投递给 out key 精确匹配的 out.
// 未注册该 key 时返回错误(由调用方决定如何处理).
// 说明: 按 RE 的"模式投递"仅在领域契约里登记设计, 等出现消费方再实现(见 domain/log/logger.go).
func (e *Engine) DeliverTo(name string, record domainlog.Record) error {
	e.mu.RLock()
	outs := e.outs
	e.mu.RUnlock()
	for _, out := range outs {
		if out.Name() != name {
			continue
		}
		e.delivered.Add(1)
		return out.Emit([]domainlog.Record{record})
	}
	return fmt.Errorf("log: out %q not registered", name)
}
