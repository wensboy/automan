package proxy

import (
	"fmt"
	"strings"
	"testing"
)

// 批量改状态: 只改选中的、未删除且状态不同的行, 返回实际改动条数并记录日志。
func TestSetStatusUpdatesSelectedOnly(t *testing.T) {
	svc, store, logger := newService()
	store.affected = 2

	changed, err := svc.SetStatus(nil, []string{"p1", " p2 ", "p1"}, StatusRunning)
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if changed != 2 {
		t.Fatalf("应返回实际改动条数 2, got %d", changed)
	}

	exec := store.lastExec()
	for _, fragment := range []string{"SET status = ?", "deleted_at = 0", "status <> ?", "id IN (?,?)"} {
		if !strings.Contains(exec.sql, fragment) {
			t.Fatalf("SQL 缺少 %q: %q", fragment, exec.sql)
		}
	}
	// 去重 + 去空白后的参数顺序: target, now, target, id1, id2
	want := []any{StatusRunning, int64(1700000000000), StatusRunning, "p1", "p2"}
	if fmt.Sprint(exec.args) != fmt.Sprint(want) {
		t.Fatalf("参数异常:\n got %v\nwant %v", exec.args, want)
	}

	record := logger.find(t, "proxy: status changed")
	if field(record, "status") != StatusRunning || field(record, "count") != "2" {
		t.Fatalf("日志字段异常: %v", record.Fields)
	}
}

// 没有任何行需要变更时不报错, 但要记一条 unchanged 日志。
func TestSetStatusNoChange(t *testing.T) {
	svc, store, logger := newService()
	store.affected = 0

	changed, err := svc.SetStatus(nil, []string{"p1"}, StatusRunning)
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if changed != 0 {
		t.Fatalf("无变更应返回 0, got %d", changed)
	}
	logger.find(t, "proxy: status unchanged")
}

// 入参校验: 非法状态 / 空选择 -> 明确错误且不写库。
func TestSetStatusValidatesInput(t *testing.T) {
	cases := []struct {
		name   string
		ids    []string
		status string
		want   string
	}{
		{"非法状态", []string{"p1"}, "starting", "status must be"},
		{"空选择", nil, StatusRunning, "no proxy selected"},
		{"全空白选择", []string{"", "  "}, StatusStopped, "no proxy selected"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			svc, store, _ := newService()
			if _, err := svc.SetStatus(nil, item.ids, item.status); err == nil ||
				!strings.Contains(err.Error(), item.want) {
				t.Fatalf("want error containing %q, got %v", item.want, err)
			}
			if len(store.execs) != 0 {
				t.Fatalf("校验失败不应写库, got %v", store.execs)
			}
		})
	}
}

// 状态统计口径: 不受状态过滤影响(否则选中某状态后其余计数恒为 0)。
func TestStatusCountsIgnoreStatusFilter(t *testing.T) {
	svc, store, _ := newService()
	store.total = 1
	store.rows = []Proxy{{ID: "p1", Name: "local-dev", Status: StatusRunning}}

	if _, err := svc.List(nil, Query{Search: "local", Status: StatusRunning}); err != nil {
		t.Fatalf("List: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	var aggregate string
	var list string
	for _, item := range store.queries {
		if strings.Contains(item.sql, "GROUP BY status") {
			aggregate = item.sql
			if len(item.args) != 3 || item.args[0] != "w-test" || item.args[1] != "%local%" {
				t.Fatalf("统计查询应只带搜索条件, got args=%v", item.args)
			}
		}
		if strings.Contains(item.sql, "LIMIT ? OFFSET ?") {
			list = item.sql
		}
	}
	if aggregate == "" {
		t.Fatalf("未执行状态统计查询")
	}
	if strings.Contains(aggregate, "status = ?") {
		t.Fatalf("统计不应被状态过滤收窄: %q", aggregate)
	}
	// 列表与总数仍然应用状态过滤
	if !strings.Contains(list, "status = ?") {
		t.Fatalf("列表应应用状态过滤: %q", list)
	}
}
