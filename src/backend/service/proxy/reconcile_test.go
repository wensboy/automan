package proxy

import (
	"strings"
	"testing"
)

// 启动/断开对账: 把所有 running 落回 stopped, 返回受影响条数。
func TestResetRunning(t *testing.T) {
	svc, store, _ := startService(t, &fakeStarter{}, `{}`)
	store.affected = 3

	if got := svc.ResetRunning(nil); got != 3 {
		t.Fatalf("应返回受影响条数 3, got %d", got)
	}
	last := store.lastExec()
	if !strings.Contains(last.sql, "status = ?") || !strings.Contains(last.sql, "deleted_at = 0") {
		t.Fatalf("对账 SQL 异常: %q", last.sql)
	}
	if len(last.args) < 3 || last.args[0] != StatusStopped || last.args[2] != StatusRunning {
		t.Fatalf("对账参数异常: %+v", last.args)
	}
}
