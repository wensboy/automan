// Package srvkit 提供 srv 领域的消费侧辅助: 类型化键的服务注册与解析.
package srvkit

import (
	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/key"
)

// Register 将服务注册到 srvs.
func Register[T any](ctx context.Context, k key.Key[T], v T) error {
	return ctx.Register(k.Name(), v)
}

// Of 从 srvs 按类型化键解析服务.
func Of[T any](ctx context.Context, k key.Key[T]) (T, bool) {
	v, ok := ctx.Srv(k.Name())
	if !ok {
		var zero T
		return zero, false
	}
	t, ok := v.(T)
	return t, ok
}
