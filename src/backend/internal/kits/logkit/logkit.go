// Package logkit 提供 log 领域的消费侧辅助: 消息/字段构造与级别名解析.
package logkit

import (
	"strings"
	"time"

	"github.com/wensboy/automan/backend/domain/log"
)

// New 构造一条带当前时间戳的日志消息.
func New(level log.Level, msg string, fields ...log.Field) log.Record {
	return log.Record{Time: time.Now(), Level: level, Msg: msg, Fields: fields}
}

// F 构造一个附加字段.
func F(key string, value any) log.Field { return log.Field{Key: key, Value: value} }

// ParseLevel 解析级别名.
func ParseLevel(s string) (log.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return log.LevelDebug, true
	case "info":
		return log.LevelInfo, true
	case "warn", "warning":
		return log.LevelWarn, true
	case "error":
		return log.LevelError, true
	default:
		return log.LevelInfo, false
	}
}
