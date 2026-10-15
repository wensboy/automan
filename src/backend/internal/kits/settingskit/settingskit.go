// Package settingskit 提供 settings 领域的消费侧辅助: 按类型化键读写运行态设置.
package settingskit

import (
	"github.com/wensboy/automan/backend/domain/key"
	"github.com/wensboy/automan/backend/domain/settings"
	"github.com/wensboy/automan/backend/internal/cast"
)

// Get 按类型化键读取生效值 (三层叠加).
func Get[T any](s settings.Store, k key.Key[T]) (T, bool) {
	v, ok := s.Get(k.Name())
	if !ok {
		var zero T
		return zero, false
	}
	return cast.As[T](v)
}

// Set 按类型化键写入运行态层.
func Set[T any](s settings.Store, k key.Key[T], v T) error {
	return s.Set(k.Name(), v)
}
