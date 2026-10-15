// Package fallback 提供"取值或回退"语义: 输入为空/非正时返回缺省值.
package fallback

import "time"

// String 返回 v; v 为空串时返回 def.
func String(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// Int 返回 v; v 非正时返回 def.
func Int(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// Int64 返回 v; v 非正时返回 def.
func Int64(v, def int64) int64 {
	if v <= 0 {
		return def
	}
	return v
}

// Duration 返回 v; v 非正时返回 def.
func Duration(v, def time.Duration) time.Duration {
	if v <= 0 {
		return def
	}
	return v
}
