package proxy

import (
	"strings"
	"testing"

	domainsettings "github.com/wensboy/automan/backend/domain/settings"
)

// fakeSettings 是 domain/settings.Store 的最小替身(只读测试用)。
type fakeSettings struct{ values map[string]any }

func (f *fakeSettings) Get(name string) (any, bool)     { v, ok := f.values[name]; return v, ok }
func (f *fakeSettings) Set(string, any) error           { return nil }
func (f *fakeSettings) Save() error                     { return nil }
func (f *fakeSettings) Reload() error                   { return nil }
func (f *fakeSettings) Reset(string) error              { return nil }
func (f *fakeSettings) View(string) domainsettings.View { return domainsettings.ViewNone }

// 创建上限: 当前工作区已有数量达到 proxy.maxCountLimit 时拒绝。
func TestCreateRespectsMaxCount(t *testing.T) {
	svc, store, _ := startService(t, &fakeStarter{}, `{}`)
	svc.WithSettings(&fakeSettings{values: map[string]any{keyMaxCount: 1}})
	store.total = 1
	store.row = nil // 避免命中"重名"分支, 只验证数量上限

	_, err := svc.Create(nil, Input{Name: "another", Rules: `{"host":"127.0.0.1","port":9000}`})
	if err == nil || !strings.Contains(err.Error(), "proxy limit reached (max 1)") {
		t.Fatalf("应命中数量上限, got %v", err)
	}
}

// 批量上限: 一次启动超过 proxy.maxStartLimit 直接拒绝(即使还没查库)。
func TestStartRespectsMaxStart(t *testing.T) {
	svc, _, _ := startService(t, &fakeStarter{}, `{}`)
	svc.WithSettings(&fakeSettings{values: map[string]any{keyMaxStart: 1}})

	_, err := svc.Start(nil, []string{"a", "b"})
	if err == nil || !strings.Contains(err.Error(), "too many proxies in one batch (max 1)") {
		t.Fatalf("应命中批量上限, got %v", err)
	}
}

// 并发上限: 已在运行的数量 + 本次启动数 超过 proxy.maxRunningLimit 时拒绝。
func TestStartRespectsMaxRunning(t *testing.T) {
	starter := &fakeStarter{}
	svc, store, _ := startService(t, starter, `{}`)
	svc.WithSettings(&fakeSettings{values: map[string]any{keyMaxRunning: 1}})
	store.total = 1 // 视作已有 1 个 running

	_, err := svc.Start(nil, []string{"p1"})
	if err == nil || !strings.Contains(err.Error(), "running limit reached (max 1") {
		t.Fatalf("应命中并发上限, got %v", err)
	}
	if len(starter.requests) != 0 {
		t.Fatal("命中上限时不应调用 starter")
	}
}

var _ domainsettings.Store = (*fakeSettings)(nil)
