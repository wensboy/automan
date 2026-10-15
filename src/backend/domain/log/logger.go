// Package log 定义统一日志的领域契约 (消息/字段构造见 internal/kits/logkit).
package log

// Logger 是统一日志调用接口: 只负责投递消息, 不做级别/目标选择.
type Logger interface {
	Deliver(record Record)
}
