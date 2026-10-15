package log

import (
	domainlog "github.com/wensboy/automan/backend/domain/log"
)

// Deliver 实现 domain/log.Logger: 分发给所有 out 后立即返回, 不阻塞投递方.
// 引擎不做任何级别过滤, 过滤策略由各 out 自持.
func (e *Engine) Deliver(record domainlog.Record) {
	e.delivered.Add(1)
	e.mu.RLock()
	outs := e.outs
	e.mu.RUnlock()
	for _, o := range outs {
		_ = o.out.Emit([]domainlog.Record{record})
	}
}
