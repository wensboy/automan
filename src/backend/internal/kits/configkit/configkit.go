// Package configkit 提供 config 领域的消费侧辅助: 按类型化键取值.
package configkit

import (
	"github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/key"
	"github.com/wensboy/automan/backend/internal/cast"
)

// Get 按类型化键从 Provider 取值并转化.
func Get[T any](p config.Provider, k key.Key[T]) (T, bool) {
	v, ok := p.Lookup(k.Name())
	if !ok {
		var zero T
		return zero, false
	}
	return cast.As[T](v)
}
