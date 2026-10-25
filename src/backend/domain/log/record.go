package log

import "time"

// Field 是日志的附加信息.
type Field struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// Record 是一条日志消息: 时间戳 + 级别 + 必要信息.
// 它只描述"是什么", 不描述"往哪写".
type Record struct {
	Time   time.Time `json:"time"`
	Level  Level     `json:"level"`
	Msg    string    `json:"msg"`
	Fields []Field   `json:"fields"`
}
