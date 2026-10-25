package log

import (
	"testing"

	domainlog "github.com/wensboy/automan/backend/domain/log"
)

// 回放环: 按游标增量返回; 容量超出丢最旧。
func TestRecentRing(t *testing.T) {
	e := New()
	e.remember([]domainlog.Record{{Msg: "a"}, {Msg: "b"}})

	items, seq := e.Recent(0, 10)
	if len(items) != 2 || items[0].Msg != "a" || seq != 2 {
		t.Fatalf("首轮回放异常: %+v seq=%d", items, seq)
	}

	newer, seq2 := e.Recent(1, 10)
	if len(newer) != 1 || newer[0].Msg != "b" || seq2 != 2 {
		t.Fatalf("游标回放异常: %+v seq=%d", newer, seq2)
	}

	none, _ := e.Recent(2, 10)
	if len(none) != 0 {
		t.Fatalf("无新记录时应为空: %+v", none)
	}

	// 容量: 追加超过 recentCapacity 时只保留最新
	batch := make([]domainlog.Record, recentCapacity+10)
	for i := range batch {
		batch[i] = domainlog.Record{Msg: "x"}
	}
	e.remember(batch)
	all, _ := e.Recent(0, recentCapacity*2)
	if len(all) != recentCapacity {
		t.Fatalf("应裁到容量上限, got %d", len(all))
	}
}

// 元信息: 最旧/最新 seq 与条数(供前端决定全量还是增量)。
func TestRecentInfo(t *testing.T) {
	e := New()
	if oldest, newest, count := e.RecentInfo(); oldest != 0 || newest != 0 || count != 0 {
		t.Fatalf("空环元信息异常: %d %d %d", oldest, newest, count)
	}
	e.remember([]domainlog.Record{{Msg: "a"}, {Msg: "b"}, {Msg: "c"}})
	oldest, newest, count := e.RecentInfo()
	if oldest != 1 || newest != 3 || count != 3 {
		t.Fatalf("元信息异常: oldest=%d newest=%d count=%d", oldest, newest, count)
	}
}
