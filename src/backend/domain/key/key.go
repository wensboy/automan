// Package key 提供跨领域复用的类型化键基元.
package key

// Key 是统一键空间中的类型化键.
type Key[T any] struct {
	name string
}

// New 创建一个类型化键.
func New[T any](name string) Key[T] { return Key[T]{name: name} }

// Name 返回键名.
func (k Key[T]) Name() string { return k.name }
