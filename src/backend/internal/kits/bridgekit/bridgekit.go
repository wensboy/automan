// Package bridgekit 提供 bridge 协议层的消费侧辅助: 负载编解码、信封、握手与处理器选项.
package bridgekit

import "github.com/wensboy/automan/backend/domain/bridge"

// WithConcurrent 声明该 name 的入站消息可并行处理 (受引擎约束).
func WithConcurrent() bridge.HandleOption {
	return func(c *bridge.HandleConfig) { c.Concurrent = true }
}
