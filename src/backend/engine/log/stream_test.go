package log

import (
	"testing"
	"time"

	domainlog "github.com/wensboy/automan/backend/domain/log"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

func rec(msg string) domainlog.Record { return logkit.New(domainlog.LevelInfo, msg) }

// 达到批量阈值时立即冲刷(不必等定时器)。
func TestStreamFlushesWhenBatchReached(t *testing.T) {
	s := newStream(streamConfig{interval: time.Hour, batch: 2, queue: 8})
	got := make(chan []domainlog.Record, 4)
	s.Subscribe(func(batch []domainlog.Record) { got <- batch })
	s.start()
	defer s.shutdown()

	if err := s.Emit([]domainlog.Record{rec("a")}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	select {
	case batch := <-got:
		t.Fatalf("未达阈值不应下发, got %d", len(batch))
	case <-time.After(30 * time.Millisecond):
	}

	if err := s.Emit([]domainlog.Record{rec("b")}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	select {
	case batch := <-got:
		if len(batch) != 2 || batch[0].Msg != "a" || batch[1].Msg != "b" {
			t.Fatalf("批量内容异常: %+v", batch)
		}
	case <-time.After(time.Second):
		t.Fatal("达到阈值应立即冲刷")
	}
}

// 未达阈值时按窗口定时下发(不逐条发)。
func TestStreamFlushesOnInterval(t *testing.T) {
	s := newStream(streamConfig{interval: 20 * time.Millisecond, batch: 128, queue: 8})
	got := make(chan []domainlog.Record, 4)
	s.Subscribe(func(batch []domainlog.Record) { got <- batch })
	s.start()
	defer s.shutdown()

	_ = s.Emit([]domainlog.Record{rec("a"), rec("b")})
	select {
	case batch := <-got:
		if len(batch) != 2 {
			t.Fatalf("应整批下发, got %d", len(batch))
		}
	case <-time.After(time.Second):
		t.Fatal("定时窗口应冲刷")
	}
}

// 溢出: 丢最旧, 并把丢弃数按"日志形式"插到批首(Warn 记录)。
func TestStreamDropsOldestAndReportsAsRecord(t *testing.T) {
	s := newStream(streamConfig{interval: 20 * time.Millisecond, batch: 128, queue: 2})
	got := make(chan []domainlog.Record, 8)
	s.Subscribe(func(batch []domainlog.Record) { got <- batch })
	s.start()
	defer s.shutdown()

	_ = s.Emit([]domainlog.Record{rec("a"), rec("b"), rec("c")}) // 容量 2 -> a 被丢
	select {
	case batch := <-got:
		if len(batch) != 3 {
			t.Fatalf("应为 1 条丢弃告警 + 2 条记录, got %d", len(batch))
		}
		if batch[0].Level != domainlog.LevelWarn || batch[0].Msg == "" {
			t.Fatalf("批首应是丢弃告警, got %+v", batch[0])
		}
		if batch[1].Msg != "b" || batch[2].Msg != "c" {
			t.Fatalf("应保留最新两条, got %q/%q", batch[1].Msg, batch[2].Msg)
		}
	case <-time.After(time.Second):
		t.Fatal("应下发一批")
	}
}

// 启动前的投递不进 stream(只转发程序运行启动后的日志)。
func TestStreamIgnoresBeforeStart(t *testing.T) {
	s := newStream(streamConfig{interval: time.Hour, batch: 1, queue: 8})
	got := make(chan []domainlog.Record, 4)
	s.Subscribe(func(batch []domainlog.Record) { got <- batch })

	_ = s.Emit([]domainlog.Record{rec("before")})
	s.start()
	defer s.shutdown()

	select {
	case batch := <-got:
		t.Fatalf("启动前记录不应下发, got %+v", batch)
	case <-time.After(30 * time.Millisecond):
	}
}

// 订阅者异常被隔离, 不影响其他订阅者。
func TestStreamIsolatesSubscriberPanic(t *testing.T) {
	s := newStream(streamConfig{interval: 20 * time.Millisecond, batch: 1, queue: 8})
	got := make(chan []domainlog.Record, 4)
	s.Subscribe(func([]domainlog.Record) { panic("boom") })
	s.Subscribe(func(batch []domainlog.Record) { got <- batch })
	s.start()
	defer s.shutdown()

	_ = s.Emit([]domainlog.Record{rec("a")})
	select {
	case batch := <-got:
		if len(batch) != 1 {
			t.Fatalf("panic 的订阅者不应影响其他订阅者, got %d", len(batch))
		}
	case <-time.After(time.Second):
		t.Fatal("正常订阅者仍应收到")
	}
}

// 取消订阅后不再收到。
func TestStreamUnsubscribe(t *testing.T) {
	s := newStream(streamConfig{interval: 20 * time.Millisecond, batch: 1, queue: 8})
	got := make(chan []domainlog.Record, 4)
	off := s.Subscribe(func(batch []domainlog.Record) { got <- batch })
	s.start()
	defer s.shutdown()

	off()
	_ = s.Emit([]domainlog.Record{rec("a")})
	select {
	case batch := <-got:
		t.Fatalf("退订后不应收到, got %+v", batch)
	case <-time.After(60 * time.Millisecond):
	}
}
