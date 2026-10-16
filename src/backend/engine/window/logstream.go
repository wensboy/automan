package window

import (
	"fmt"

	domainlog "github.com/wensboy/automan/backend/domain/log"
)

// 前端可见的日志记录形态: 直接用 Record 的 JSON 形式
// (time/level/msg/fields, 见 docs/design/v16.md 第 6 节与第 10 节 F)。
type streamRecord struct {
	Time   string        `json:"time"`
	Level  string        `json:"level"`
	Msg    string        `json:"msg"`
	Fields []streamField `json:"fields"`
}

type streamField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// forwardLogStream 是日志二级总线的订阅者: 把一批记录转成前端事件。
// 订阅发生在引擎 Init, 而事件通道在窗口就绪后才可用, 因此此处对未就绪状态直接丢弃。
func (e *Engine) forwardLogStream(batch []domainlog.Record) {
	if e.app == nil || len(batch) == 0 {
		return
	}
	e.app.Event.Emit("log:stream", toStreamPayload(batch))
}

func toStreamPayload(batch []domainlog.Record) []streamRecord {
	out := make([]streamRecord, 0, len(batch))
	for _, record := range batch {
		fields := make([]streamField, 0, len(record.Fields))
		for _, field := range record.Fields {
			fields = append(fields, streamField{Key: field.Key, Value: fmt.Sprint(field.Value)})
		}
		out = append(out, streamRecord{
			Time:   record.Time.Format("2006-01-02T15:04:05.000Z07:00"),
			Level:  record.Level.String(),
			Msg:    record.Msg,
			Fields: fields,
		})
	}
	return out
}
