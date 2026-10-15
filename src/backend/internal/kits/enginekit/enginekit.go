// Package enginekit 提供 engine 领域的消费侧辅助: 按名称解析引擎.
package enginekit

import "github.com/wensboy/automan/backend/domain/context"

// Of 按名称取引擎并做类型断言.
func Of[T any](ctx context.Context, name string) (T, bool) {
	v, ok := ctx.Engine(name)
	if !ok {
		var zero T
		return zero, false
	}
	t, ok := v.(T)
	return t, ok
}
