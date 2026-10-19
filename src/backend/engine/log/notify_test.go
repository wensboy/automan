package log

import (
	"fmt"
	"testing"

	domainlog "github.com/wensboy/automan/backend/domain/log"
)

// 环形: 超容量丢最旧, 最新在前, 每条都有 id; 清空后为空。
func TestNotifyRingAndClear(t *testing.T) {
	published := make([]domainlog.Notice, 0, 5)
	out := newNotifyOut(3, func(notice domainlog.Notice) { published = append(published, notice) })

	for i := 0; i < 5; i++ {
		title := fmt.Sprintf("t%d", i)
		err := out.Emit([]domainlog.Record{{
			Msg: title,
			Fields: []domainlog.Field{
				{Key: noticeKindField, Value: noticeKindValue}, // 与 Notify() 组装的一致
				{Key: "label", Value: "proxy"},
				{Key: "content", Value: "detail"},
			},
		}})
		if err != nil {
			t.Fatalf("Emit %s: %v", title, err)
		}
	}

	items := out.list()
	if len(items) != 3 {
		t.Fatalf("容量应为 3, got %d", len(items))
	}
	if items[0].Title != "t4" || items[2].Title != "t2" {
		t.Fatalf("应保留最新三条且最新在前, got %+v", items)
	}
	if items[0].ID == "" || items[0].Label != "proxy" || items[0].Content != "detail" {
		t.Fatalf("字段映射异常: %+v", items[0])
	}
	if len(published) != 5 {
		t.Fatalf("每条都应发布事件, got %d", len(published))
	}

	out.clear()
	if len(out.list()) != 0 {
		t.Fatal("清空后应为空")
	}
}

// 普通日志(无 notice 标记)不得进入提示队列 —— 引擎扇出投递会把所有日志送到这里。
func TestNotifyIgnoresPlainLogs(t *testing.T) {
	out := newNotifyOut(10, nil)
	if err := out.Emit([]domainlog.Record{{Msg: "bridge: asset ready"}}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(out.list()) != 0 {
		t.Fatalf("普通日志不应入队, got %+v", out.list())
	}
}
