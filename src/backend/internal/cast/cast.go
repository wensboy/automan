// Package cast 提供跨领域的类型转化, 后续所有转化统一在此定义.
package cast

import "encoding/json"

// As 将 v 转化为 T: 优先直接断言, 否则经 JSON 往返兼容结构差异.
func As[T any](v any) (T, bool) {
	var zero T
	if v == nil {
		return zero, false
	}
	if t, ok := v.(T); ok {
		return t, true
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return zero, false
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, false
	}
	return out, true
}
