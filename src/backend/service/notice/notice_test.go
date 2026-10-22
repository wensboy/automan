package notice

import (
	"testing"

	"github.com/wensboy/automan/backend/domain/event"
)

type fakeBus struct {
	topics   []event.Topic
	payloads []any
}

func (b *fakeBus) Emit(topic event.Topic, payload any) {
	b.topics = append(b.topics, topic)
	b.payloads = append(b.payloads, payload)
}

func (b *fakeBus) Subscribe(event.Topic, func(any)) func() { return func() {} }

// 投递: 标题必填; 入队最新在前; 广播 notice.appended。
func TestPushAppendsAndBroadcasts(t *testing.T) {
	bus := &fakeBus{}
	svc := New().WithEventBus(bus)

	if _, err := svc.Push(Entry{Title: "  "}); err == nil {
		t.Fatal("空标题应被拒绝")
	}

	first, err := svc.Push(Entry{Title: "第一条", Level: "warn", Label: "proxy", Content: "细节"})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	second, _ := svc.Push(Entry{Title: "第二条", Level: "error"})
	if second.Level != "error" {
		t.Fatalf("level 应保留, got %q", second.Level)
	}
	if first.ID == second.ID || first.ID == "" {
		t.Fatalf("id 应唯一且非空: %q / %q", first.ID, second.ID)
	}

	items := svc.List(nil)
	if len(items) != 2 || items[0].Title != "第二条" || items[1].Title != "第一条" {
		t.Fatalf("队列应最新在前, got %+v", items)
	}
	if len(bus.topics) != 2 || bus.topics[0] != event.TopicNoticeAppended {
		t.Fatalf("应广播 notice.appended, got %+v", bus.topics)
	}
	if payload, ok := bus.payloads[0].(event.NoticeAppended); !ok || payload.Title != "第一条" {
		t.Fatalf("载荷异常: %#v", bus.payloads[0])
	}
}

// 环形容量: 超出丢最旧; Clear 清空并广播 notice.cleared。
func TestCapacityAndClear(t *testing.T) {
	bus := &fakeBus{}
	svc := New().WithEventBus(bus).WithCapacity(2)

	for _, title := range []string{"a", "b", "c"} {
		if _, err := svc.Push(Entry{Title: title}); err != nil {
			t.Fatalf("Push(%s): %v", title, err)
		}
	}
	items := svc.List(nil)
	if len(items) != 2 || items[0].Title != "c" || items[1].Title != "b" {
		t.Fatalf("容量 2 时应保留最新两条, got %+v", items)
	}

	svc.Clear(nil)
	if len(svc.List(nil)) != 0 {
		t.Fatal("Clear 后应为空")
	}
	if last := bus.topics[len(bus.topics)-1]; last != event.TopicNoticeCleared {
		t.Fatalf("Clear 应广播 notice.cleared, got %q", last)
	}
}

// 勿扰: 不排队、不广播(避免"关掉勿扰后一次性刷出积压提示")。
func TestDoNotDisturbSuppresses(t *testing.T) {
	bus := &fakeBus{}
	enabled := true
	svc := New().WithEventBus(bus).WithDND(func() bool { return enabled })

	entry, err := svc.Push(Entry{Title: "不该出现", Level: "warn"})
	if err != nil {
		t.Fatalf("勿扰时不应报错: %v", err)
	}
	if entry.ID != "" {
		t.Fatalf("被抑制的提示不应返回 id: %+v", entry)
	}
	if len(svc.List(nil)) != 0 || len(bus.topics) != 0 {
		t.Fatalf("勿扰时不应入队/广播: items=%d topics=%d", len(svc.List(nil)), len(bus.topics))
	}

	// 关闭勿扰后恢复正常
	enabled = false
	if _, err := svc.Push(Entry{Title: "可以出现"}); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(svc.List(nil)) != 1 || len(bus.topics) != 1 {
		t.Fatalf("关闭勿扰后应正常入队/广播: items=%d topics=%d", len(svc.List(nil)), len(bus.topics))
	}
}
