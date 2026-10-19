package log

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wensboy/automan/backend/domain/event"
	domainlog "github.com/wensboy/automan/backend/domain/log"
)

// notifyCapacity 是提示队列上限: 超出丢最旧(提示不是审计, 不做丢弃上报).
//
//nolint:gochecknoglobals // 配置项未开放前的固定容量
const notifyCapacity = 200

// notifyOut 是运行时提示的出口: 内存环形 + 事件转发, **不落盘**.
type notifyOut struct {
	mu       sync.Mutex
	items    []domainlog.Notice // 最新在前
	capacity int
	seq      int64
	publish  func(domainlog.Notice)
}

func newNotifyOut(capacity int, publish func(domainlog.Notice)) *notifyOut {
	if capacity <= 0 {
		capacity = notifyCapacity
	}
	return &notifyOut{capacity: capacity, publish: publish}
}

// Name 实现 domainlog.Out.
func (o *notifyOut) Name() string { return domainlog.OutNotify }

// Emit 实现 domainlog.Out: 由 Notify 组装好的记录转回 Notice 后入环形.
func (o *notifyOut) Emit(records []domainlog.Record) error {
	for _, record := range records {
		// 只接收 Notify() 产生的记录: 引擎的扇出投递(Deliver)会把**所有普通日志**也送到这里,
		// 若不区分, "bridge: asset ready" 这类日志就会变成无 label 的提示(实测踩到)。
		if !isNoticeRecord(record) {
			continue
		}
		notice := domainlog.Notice{Time: record.Time, Level: record.Level, Title: record.Msg}
		for _, field := range record.Fields {
			value, _ := field.Value.(string)
			switch field.Key {
			case "label":
				notice.Label = value
			case "content":
				notice.Content = value
			case "noticeId":
				notice.ID = value
			}
		}
		o.append(notice)
	}
	return nil
}

func (o *notifyOut) append(notice domainlog.Notice) {
	o.mu.Lock()
	if notice.Time.IsZero() {
		notice.Time = time.Now()
	}
	if notice.ID == "" {
		o.seq++
		notice.ID = fmt.Sprintf("%d-%d", notice.Time.UnixNano(), o.seq)
	}
	o.items = append([]domainlog.Notice{notice}, o.items...)
	if len(o.items) > o.capacity {
		o.items = o.items[:o.capacity]
	}
	snapshot := notice
	o.mu.Unlock()

	if o.publish != nil {
		o.publish(snapshot)
	}
}

func (o *notifyOut) list() []domainlog.Notice {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]domainlog.Notice, len(o.items))
	copy(out, o.items)
	return out
}

func (o *notifyOut) clear() {
	o.mu.Lock()
	o.items = nil
	o.mu.Unlock()
}

// noticeKindField 是"这条记录来自 Notify()"的标记键(值固定为 noticeKindValue).
const (
	noticeKindField = "noticeKind"
	noticeKindValue = "notice"
)

func isNoticeRecord(record domainlog.Record) bool {
	for _, field := range record.Fields {
		if field.Key == noticeKindField {
			value, _ := field.Value.(string)
			return value == noticeKindValue
		}
	}
	return false
}

// Notify 实现 domainlog.Notifier: 组装成一条特殊日志, 定向投递给 notify out.
func (e *Engine) Notify(notice domainlog.Notice) {
	if e.notify == nil {
		return
	}
	// 标题是"给人看的一句话结论": 空标题会渲染成一条空壳, 直接丢弃(调用方应走日志)
	if strings.TrimSpace(notice.Title) == "" {
		return
	}
	record := domainlog.Record{
		Time:  notice.Time,
		Level: notice.Level,
		Msg:   notice.Title,
		Fields: []domainlog.Field{
			{Key: noticeKindField, Value: noticeKindValue},
			{Key: "label", Value: notice.Label},
			{Key: "content", Value: notice.Content},
			{Key: "noticeId", Value: notice.ID},
		},
	}
	// 定向投递: 不进 disk/stream, 避免污染日志文件与终端
	_ = e.DeliverTo(domainlog.OutNotify, record)
}

// Notices 实现 domainlog.Notifier: 当前队列(最新在前).
func (e *Engine) Notices() []domainlog.Notice {
	if e.notify == nil {
		return nil
	}
	return e.notify.list()
}

// ClearNotices 实现 domainlog.Notifier: 清空队列并广播(所有窗口同步清空).
func (e *Engine) ClearNotices() {
	if e.notify == nil {
		return
	}
	e.notify.clear()
	if e.ebus != nil {
		e.ebus.Emit(event.TopicNoticeCleared, event.NoticeCleared{At: time.Now().UnixMilli()})
	}
}
