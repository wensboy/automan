package log

import "time"

// Field 是日志的附加信息.
type Field struct {
	Key   string
	Value any
}

// Record 是一条日志消息: 时间戳 + 级别 + 必要信息.
// 它只描述"是什么", 不描述"往哪写".
type Record struct {
	Time   time.Time
	Level  Level
	Msg    string
	Fields []Field
}
