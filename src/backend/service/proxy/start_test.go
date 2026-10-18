package proxy

import (
	stdctx "context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// fakeStarter 记录请求, 并按预设值回传"实际生效"参数。
type fakeStarter struct {
	requests []StartRequest
	stopped  []string
	info     StartInfo
	err      error
}

func (f *fakeStarter) Start(_ stdctx.Context, req StartRequest) (StartInfo, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return StartInfo{}, f.err
	}
	info := f.info
	if info.Port == 0 {
		info.Port = req.Port
	}
	if info.UIPort == 0 {
		info.UIPort = req.UIPort
	}
	return info, nil
}

func (f *fakeStarter) Stop(_ stdctx.Context, proxyID string, _ time.Duration) error {
	f.stopped = append(f.stopped, proxyID)
	if f.err != nil {
		return f.err
	}
	return nil
}

func startService(t *testing.T, starter Starter, template string) (*Service, *fakeStore, *fakeLogger) {
	t.Helper()
	store := &fakeStore{total: 1, affected: 1}
	logger := &fakeLogger{}
	svc := New(store, logger,
		WithStarter(starter),
		WithData(fstest.MapFS{
			"data/default/proxy.rules.json": &fstest.MapFile{Data: []byte(template)},
			"data/conf/bridge.json": &fstest.MapFile{Data: []byte(
				`{"node":{"services":{"proxy":{"root":"proxies","stopTimeoutMs":5000}}}}`)},
		}),
	)
	svc.now = func() time.Time { return time.UnixMilli(1700000000000) }
	svc.newID = func() string { return "p1" }
	return svc, store, logger
}

// 启动成功: 参数以 rules 为准, 让步后的 storage/target 写回数据库。
func TestStartWritesBackYieldingResult(t *testing.T) {
	starter := &fakeStarter{}
	svc, store, logger := startService(t, starter, `{"host":"127.0.0.1","port":"8899","uiport":"8900","storage":"proxies/demo","timeout":30000,"dnsServer":""}`)
	store.row = &Proxy{ID: "p1", Name: "demo", Target: "old:1", Status: StatusStopped,
		Rules: `{"host":"127.0.0.1","port":"8899","uiport":"8900","storage":"proxies/demo","timeout":30000,"dnsServer":""}`}

	result, err := svc.Start(nil, []string{"p1"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(result.Items) != 1 || len(result.Failed) != 0 {
		t.Fatalf("结果异常: %+v", result)
	}
	item := result.Items[0]
	// 端口可能因占用而让步, 所以断言"target 与最终端口一致"
	if want := fmt.Sprintf("127.0.0.1:%d", item.Port); item.Target != want {
		t.Fatalf("target 应为 host:port(%s), got %q", want, item.Target)
	}
	// 写回: rules 含 storage/port, target 同步, status=running
	exec := store.lastExec()
	for _, fragment := range []string{"UPDATE proxies SET rules = ?", "target = ?", "status = ?"} {
		if !strings.Contains(exec.sql, fragment) {
			t.Fatalf("写回 SQL 缺少 %q: %q", fragment, exec.sql)
		}
	}
	if exec.args[1] != item.Target || exec.args[2] != StatusRunning {
		t.Fatalf("写回参数异常: %v", exec.args)
	}
	logger.find(t, "proxy: started")
}

// node 回传的实际端口优先。
func TestStartUsesStarterEffectiveValues(t *testing.T) {
	starter := &fakeStarter{info: StartInfo{Port: 9001, UIPort: 9002, PID: 42}}
	svc, store, _ := startService(t, starter, `{}`)
	store.row = &Proxy{ID: "p1", Name: "demo", Status: StatusStopped, Rules: `{"host":"127.0.0.1","port":8899,"uiport":8899}`}

	result, err := svc.Start(nil, []string{"p1"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	item := result.Items[0]
	if item.Port != 9001 || item.UIPort != 9002 || item.Target != "127.0.0.1:9001" || item.PID != 42 {
		t.Fatalf("应采用 node 回传的实际值, got %+v", item)
	}
}

// 启动失败: 该行置 error 并进入 failed 列表。
func TestStartFailureMarksError(t *testing.T) {
	starter := &fakeStarter{err: errTest{msg: "whistle: not installed"}}
	svc, store, logger := startService(t, starter, `{}`)
	store.row = &Proxy{ID: "p1", Name: "demo", Status: StatusStopped, Rules: `{"host":"127.0.0.1","port":8899}`}

	result, err := svc.Start(nil, []string{"p1"})
	if err != nil {
		t.Fatalf("Start 不应整体失败: %v", err)
	}
	if len(result.Failed) != 1 || !strings.Contains(result.Failed[0].Reason, "not installed") {
		t.Fatalf("失败原因异常: %+v", result.Failed)
	}
	logger.find(t, "proxy: failed")
}

// 未配置执行链路时明确报错, 不静默成功。
func TestStartWithoutStarterFails(t *testing.T) {
	svc := New(&fakeStore{total: 1}, &fakeLogger{})
	if _, err := svc.Start(nil, []string{"p1"}); err == nil ||
		!strings.Contains(err.Error(), "starter not configured") {
		t.Fatalf("应报 starter not configured, got %v", err)
	}
}

// basedir 按代理名隔离: <proxies>/<name>(name 库内唯一, 不再需要随机后缀让步)。
func TestPrepareStartUsesPerProxyBaseDir(t *testing.T) {
	svc, _, _ := startService(t, &fakeStarter{}, `{"host":"127.0.0.1","port":8899,"uiport":8899}`)
	svc.proxyRoot = svc.resolveBase("proxies")

	row := Proxy{ID: "p1", Name: "test_proxy_1", Rules: `{"host":"127.0.0.1","port":8899,"uiport":8899}`}
	req, err := svc.prepareStartRequest(row)
	if err != nil {
		t.Fatalf("prepareStartRequest: %v", err)
	}
	if want := filepath.Join(svc.proxyRoot, "test_proxy_1"); req.BaseDir != want {
		t.Fatalf("basedir 应为 %q, got %q", want, req.BaseDir)
	}
	// 同一行多次准备结果稳定(没有随机后缀)
	again, _ := svc.prepareStartRequest(row)
	if again.BaseDir != req.BaseDir {
		t.Fatalf("basedir 不应随调用变化: %q vs %q", req.BaseDir, again.BaseDir)
	}
	if req.Port == req.UIPort {
		t.Fatalf("port 与 uiport 不应相同, got %d", req.Port)
	}
}

// 停止: 调 starter 并置 stopped, 同时释放映射。
func TestStopUpdatesStatus(t *testing.T) {
	starter := &fakeStarter{}
	svc, store, logger := startService(t, starter, `{}`)
	store.row = &Proxy{ID: "p1", Name: "demo", Status: StatusRunning}

	if err := svc.Stop(nil, []string{"p1"}); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(starter.stopped) != 1 || starter.stopped[0] != "p1" {
		t.Fatalf("应调用 starter.Stop, got %v", starter.stopped)
	}
	logger.find(t, "proxy: stopped")
}

// 模板: 原样返回(供前端编辑器)。
func TestTemplatePassthrough(t *testing.T) {
	svc, _, _ := startService(t, &fakeStarter{}, `{"host":"127.0.0.1"}`)
	if got := svc.Template(nil); got != `{"host":"127.0.0.1"}` {
		t.Fatalf("模板应原样返回, got %q", got)
	}
}

type errTest struct{ msg string }

func (e errTest) Error() string { return e.msg }

// basedir 必须是**绝对路径**: 相对路径会成为 node 侧 spawn 的 cwd, 目录不存在时报 ENOENT。
func TestPrepareStartUsesAbsolutePaths(t *testing.T) {
	svc, _, _ := startService(t, &fakeStarter{}, `{"host":"127.0.0.1","port":8899,"uiport":8899}`)
	svc.root = t.TempDir()
	svc.proxyRoot = svc.resolveBase("proxies")

	req, err := svc.prepareStartRequest(Proxy{ID: "p1", Name: "demo",
		Rules: `{"host":"127.0.0.1","port":8899,"uiport":8899}`})
	if err != nil {
		t.Fatalf("prepareStartRequest: %v", err)
	}
	wantBase := filepath.Join(svc.root, "proxies", "demo")
	if req.BaseDir != wantBase || !filepath.IsAbs(req.BaseDir) {
		t.Fatalf("basedir 应为绝对路径 %q, got %q", wantBase, req.BaseDir)
	}
}

// 启动期目录准备: basedir 由 bootstrap 触发创建, 且必须幂等。
func TestEnsureBaseDirCreatesDirectory(t *testing.T) {
	svc, _, logger := startService(t, &fakeStarter{}, `{}`)
	svc.root = t.TempDir()
	svc.proxyRoot = svc.resolveBase("proxies")

	if _, err := os.Stat(svc.proxyRoot); err == nil {
		t.Fatalf("前置条件不成立: %s 不应存在", svc.proxyRoot)
	}
	if err := svc.EnsureBaseDir(); err != nil {
		t.Fatalf("EnsureBaseDir: %v", err)
	}
	info, err := os.Stat(svc.proxyRoot)
	if err != nil || !info.IsDir() {
		t.Fatalf("basedir 应被创建为目录: err=%v", err)
	}
	// 幂等: 再次调用不报错
	if err := svc.EnsureBaseDir(); err != nil {
		t.Fatalf("重复调用应无副作用: %v", err)
	}
	logger.find(t, "proxy: basedir ready")
}

// 改名: 旧 basedir 迁到新名字下; 运行中或目标已存在时跳过(只告警, 不动数据)。
func TestMigrateBaseDirOnRename(t *testing.T) {
	svc, _, logger := startService(t, &fakeStarter{}, `{}`)
	svc.root = t.TempDir()
	svc.proxyRoot = svc.resolveBase("proxies")

	oldDir := svc.baseDirOf("old-name")
	if err := os.MkdirAll(filepath.Join(oldDir, ".whistle", "rules"), 0o755); err != nil {
		t.Fatalf("准备旧目录: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, ".whistle", "rules", "default.rules"), []byte("x"), 0o644); err != nil {
		t.Fatalf("准备旧数据: %v", err)
	}

	// 运行中: 跳过, 旧目录仍在
	svc.migrateBaseDir("old-name", "new-name", true)
	if _, err := os.Stat(filepath.Join(oldDir, ".whistle")); err != nil {
		t.Fatalf("运行中不应迁移: %v", err)
	}
	logger.find(t, "proxy: basedir migration skipped (running)")

	// 正常改名: 目录与会随之迁移
	svc.migrateBaseDir("old-name", "new-name", false)
	newDir := svc.baseDirOf("new-name")
	if _, err := os.Stat(filepath.Join(newDir, ".whistle", "rules", "default.rules")); err != nil {
		t.Fatalf("应迁移到新目录: %v", err)
	}
	if _, err := os.Stat(oldDir); err == nil {
		t.Fatalf("旧目录应已不存在: %s", oldDir)
	}
	logger.find(t, "proxy: basedir migrated")

	// 目标已存在: 跳过且保留双方
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("重建旧目录: %v", err)
	}
	svc.migrateBaseDir("old-name", "new-name", false)
	logger.find(t, "proxy: basedir migration skipped (target exists)")
}

// capture 目标: 只收 status=running 且 rules.uiport>0 的行, host 为通配地址时换成本机地址。
func TestRunningReturnsCaptureTargets(t *testing.T) {
	svc, store, _ := startService(t, &fakeStarter{}, `{}`)
	store.row = &Proxy{ID: "p1", Name: "demo", Status: StatusRunning,
		Rules: `{"host":"0.0.0.0","uiport":8899,"port":8899}`}

	targets, err := svc.Running(nil)
	if err != nil {
		t.Fatalf("Running: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("应返回 1 个目标, got %+v", targets)
	}
	if targets[0].URL != "http://127.0.0.1:8899" || targets[0].UIPort != 8899 {
		t.Fatalf("通配地址应换成本机地址, got %+v", targets[0])
	}

	// 没配 uiport: 不加入列表
	store.row = &Proxy{ID: "p2", Name: "no-ui", Status: StatusRunning, Rules: `{"host":"127.0.0.1","port":8899}`}
	if got, err := svc.Running(nil); err != nil || len(got) != 0 {
		t.Fatalf("未配置 uiport 不应返回, got %+v err=%v", got, err)
	}
}
