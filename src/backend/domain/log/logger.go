// Package log 定义统一日志的领域契约 (消息/字段构造见 internal/kits/logkit).
package log

// Logger 是统一日志调用接口: 只负责投递消息, 不做级别/目标选择.
//
// 投递模式(见 docs/design/v16.md 第 4 节):
//   - 扇出投递 Deliver      : 分发给所有 out (现状, 保留)
//   - 定向投递 DeliverTo    : 只投递给 Name() 精确匹配的 out (已实现)
//   - 模式投递 DeliverPattern: 按 RE 匹配投递 —— **仅登记设计**, 等出现消费方再实现
type Logger interface {
	// Deliver 扇出投递: 分发给所有 out.
	Deliver(record Record)
	// DeliverTo 定向投递: 只投递给 out key 精确匹配的 out; 未注册该 key 时返回错误.
	DeliverTo(name string, record Record) error
}
