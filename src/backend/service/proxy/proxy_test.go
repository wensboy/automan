package proxy

import (
	stdctx "context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
)

// ---- 测试替身 ----

type call struct {
	sql  string
	args []any
}

// fakeStore 是内存版 store.Store: 按 SQL 形态返回预置结果, 并记录调用参数。
type fakeStore struct {
	mu       sync.Mutex
	total    int
	row      *Proxy
	rows     []Proxy
	byID     map[string]Proxy
	affected int64

	execs   []call
	queries []call
	cmds    []domainstore.Cmd
}

func (f *fakeStore) Submit(domainstore.Task) <-chan domainstore.Result {
	ch := make(chan domainstore.Result, 1)
	ch <- domainstore.Result{}
	close(ch)
	return ch
}

func (f *fakeStore) Do(_ stdctx.Context, task domainstore.Task) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cmds = append(f.cmds, task.Cmd)

	switch task.Cmd {
	case domainstore.CmdExec:
		args, _ := task.Args.(domainstore.ExecArgs)
		f.execs = append(f.execs, call{sql: args.SQL, args: args.Args})
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(args.SQL)), "INSERT") {
			// 把 INSERT 的参数落成一行, 供随后的 Get 使用
			f.row = &Proxy{
				ID: args.Args[0].(string), Name: args.Args[1].(string), Target: args.Args[2].(string),
				Status: args.Args[3].(string), Rules: args.Args[4].(string), Description: args.Args[5].(string),
				CreatedAt: args.Args[6].(int64), UpdatedAt: args.Args[7].(int64),
			}
		}
		// 只处理"编辑"这条 UPDATE; 逻辑删除的 UPDATE 参数不同, 不在此解析
		// 参数顺序(name,target,rules,description,updated_at,id): status 不在入参内(由执行链路维护)
		if strings.Contains(args.SQL, "SET name = ?") && f.row != nil && len(args.Args) >= 6 {
			f.row.Name = args.Args[0].(string)
			f.row.Target = args.Args[1].(string)
			f.row.Rules = args.Args[2].(string)
			f.row.Description = args.Args[3].(string)
			f.row.UpdatedAt = args.Args[4].(int64)
		}
		return domainstore.ExecResult{RowsAffected: f.affected, LastInsertID: 0}, nil

	case domainstore.CmdQuery:
		args, _ := task.Args.(domainstore.QueryArgs)
		f.queries = append(f.queries, call{sql: args.SQL, args: args.Args})

		switch {
		case strings.Contains(args.SQL, "GROUP BY status"):
			// 页脚统计: 返回通用行集
			return domainstore.QueryResult{Count: 1, Rows: []domainstore.Row{
				{"status": StatusRunning, "total": int64(3)},
				{"status": StatusError, "total": int64(1)},
			}}, nil

		case strings.Contains(args.SQL, "COUNT(*)"):
			if into, ok := args.Into.(*[]int); ok {
				*into = []int{f.total}
			}
			return domainstore.QueryResult{Count: 1}, nil

		case strings.Contains(args.SQL, "LIMIT ? OFFSET ?"):
			if into, ok := args.Into.(*[]Proxy); ok {
				*into = append([]Proxy{}, f.rows...)
			}
			return domainstore.QueryResult{Count: len(f.rows)}, nil

		default: // 按 id 取单行(引擎的 Into 只支持切片, 所以这里也返回切片)
			if into, ok := args.Into.(*[]Proxy); ok {
				switch {
				case f.byID != nil && len(args.Args) > 0:
					if id, ok := args.Args[len(args.Args)-1].(string); ok {
						if row, exists := f.byID[id]; exists {
							*into = []Proxy{row}
							return domainstore.QueryResult{Count: 1}, nil
						}
					}
					*into = []Proxy{}
				case f.row == nil:
					*into = []Proxy{}
				default:
					*into = []Proxy{*f.row}
				}
			}
			return domainstore.QueryResult{Count: 1}, nil
		}
	}
	return nil, fmt.Errorf("fakeStore: unsupported cmd %v", task.Cmd)
}

func (f *fakeStore) lastExec() call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.execs[len(f.execs)-1]
}

func (f *fakeStore) lastQuery() call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.queries[len(f.queries)-1]
}

// fakeLogger 收集日志, 便于断言"必要操作记录"。
type fakeLogger struct {
	mu      sync.Mutex
	records []domainlog.Record
}

func (f *fakeLogger) Deliver(record domainlog.Record) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, record)
}

func (f *fakeLogger) DeliverTo(string, domainlog.Record) error { return nil }

func (f *fakeLogger) find(t *testing.T, msg string) domainlog.Record {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, record := range f.records {
		if record.Msg == msg {
			return record
		}
	}
	t.Fatalf("未找到日志 %q, 实际: %v", msg, f.records)
	return domainlog.Record{}
}

func field(record domainlog.Record, key string) string {
	for _, item := range record.Fields {
		if item.Key == key {
			return fmt.Sprint(item.Value)
		}
	}
	return ""
}

func newService() (*Service, *fakeStore, *fakeLogger) {
	store := &fakeStore{affected: 1}
	logger := &fakeLogger{}
	svc := New().WithStore(store).WithLogger(logger)
	// 资源操作要求"有当前工作区"(v21 §10): 测试默认给一个
	svc.currentWorkspace = func() string { return "w-test" }
	svc.now = func() time.Time { return time.UnixMilli(1700000000000) }
	svc.newID = func() string { return "test-id-1" }
	return svc, store, logger
}

// ---- 校验 ----

func TestCreateValidatesInput(t *testing.T) {
	cases := []struct {
		name  string
		input Input
		want  string
	}{
		{"空 name", Input{Name: "  ", Target: "127.0.0.1:8899"}, "name must be"},
		{"超长 name", Input{Name: strings.Repeat("a", maxNameLen+1), Target: "t"}, "name must be"},
		{"rules 过大", Input{Name: "p", Target: "t", Rules: strings.Repeat("x", maxTextLen+1)}, "rules exceeds"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			svc, store, logger := newService()
			if _, err := svc.Create(nil, item.input); err == nil || !strings.Contains(err.Error(), item.want) {
				t.Fatalf("want error containing %q, got %v", item.want, err)
			}
			if len(store.execs) != 0 {
				t.Fatalf("校验失败不应写库, got %v", store.execs)
			}
			logger.find(t, "proxy: rejected")
		})
	}
}

// ---- 创建 ----

func TestCreateInsertsAndLogs(t *testing.T) {
	svc, store, logger := newService()

	// target 不再由前端提供: 由 rules 的 host/port 派生(传进来的 Target 会被忽略)
	created, err := svc.Create(nil, Input{Name: " local-dev ", Target: "ignored",
		Rules: `{"host":"127.0.0.1","port":8899}`})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	exec := store.lastExec()
	if !strings.HasPrefix(strings.TrimSpace(strings.ToUpper(exec.sql)), "INSERT") {
		t.Fatalf("应写入 INSERT, got %q", exec.sql)
	}
	// 归一化: 首尾空白被裁掉; status 缺省 stopped; 时间来自注入的时钟
	if exec.args[0] != "test-id-1" || exec.args[1] != "local-dev" || exec.args[2] != "127.0.0.1:8899" {
		t.Fatalf("插入参数异常: %v", exec.args)
	}
	if exec.args[3] != StatusStopped || exec.args[6] != int64(1700000000000) {
		t.Fatalf("缺省状态/时间异常: %v", exec.args)
	}
	if created.ID != "test-id-1" || created.Name != "local-dev" {
		t.Fatalf("返回行异常: %+v", created)
	}
	record := logger.find(t, "proxy: created")
	if field(record, "id") != "test-id-1" || field(record, "name") != "local-dev" {
		t.Fatalf("创建日志字段异常: %v", record.Fields)
	}
}

func TestCreateRejectsDuplicateName(t *testing.T) {
	svc, store, logger := newService()
	store.total = 1 // 同名未删除行已存在

	if _, err := svc.Create(nil, Input{Name: "dup", Target: "t"}); err == nil || !strings.Contains(err.Error(), "duplicate name") {
		t.Fatalf("重复名应被拒绝, got %v", err)
	}
	if len(store.execs) != 0 {
		t.Fatalf("重复名不应写库")
	}
	logger.find(t, "proxy: rejected")
}

// ---- 读取 ----

func TestGetNotFound(t *testing.T) {
	svc, _, _ := newService()
	if _, err := svc.Get(nil, ""); err == nil {
		t.Fatalf("空 id 应报未找到")
	}
	if _, err := svc.Get(nil, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("不存在的行应报未找到, got %v", err)
	}
}

// ---- 更新 ----

func TestUpdateWritesChangedFieldsLog(t *testing.T) {
	svc, store, logger := newService()
	store.row = &Proxy{ID: "p1", Name: "old", Target: "t", Status: StatusStopped, Rules: "{}"}

	updated, err := svc.Update(nil, "p1", Input{Name: "new", Target: "t", Rules: "{}"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	exec := store.lastExec()
	if !strings.HasPrefix(strings.TrimSpace(strings.ToUpper(exec.sql)), "UPDATE") {
		t.Fatalf("应写入 UPDATE, got %q", exec.sql)
	}
	if !strings.Contains(exec.sql, "deleted_at = 0") {
		t.Fatalf("更新不应波及已删除行: %q", exec.sql)
	}
	record := logger.find(t, "proxy: updated")
	if got := field(record, "fields"); got != "name,target" {
		t.Fatalf("改动字段应为 name,target(rules 变化会带动派生 target; status 不在入参内), got %q", got)
	}
	if updated.Name != "new" {
		t.Fatalf("返回行异常: %+v", updated)
	}
}

// ---- 删除(逻辑删除) ----

func TestDeleteIsSoftDelete(t *testing.T) {
	svc, store, logger := newService()
	store.row = &Proxy{ID: "p1", Name: "local-dev"}

	if err := svc.Delete(nil, "p1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	exec := store.lastExec()
	if !strings.Contains(strings.ToUpper(exec.sql), "UPDATE PROXIES SET DELETED_AT") {
		t.Fatalf("应为逻辑删除, got %q", exec.sql)
	}
	if exec.args[0] != int64(1700000000000) {
		t.Fatalf("deleted_at 应取注入时钟, got %v", exec.args[0])
	}
	logger.find(t, "proxy: deleted")
}

func TestDeleteMissingReportsNotFound(t *testing.T) {
	svc, store, _ := newService()
	store.row = &Proxy{ID: "p1", Name: "x"}
	store.affected = 0 // 已被他人删除

	if err := svc.Delete(nil, "p1"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("影响行数为 0 应报未找到, got %v", err)
	}
}

// ---- 列表: 过滤/搜索/分页 ----

func TestListBuildsFilterAndPaging(t *testing.T) {
	svc, store, _ := newService()
	store.total = 42
	store.rows = []Proxy{{ID: "p1", Name: "local-dev"}}

	page, err := svc.List(nil, Query{Search: "local", Status: StatusRunning, Page: 3, PageSize: 5})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 42 || len(page.Items) != 1 {
		t.Fatalf("列表结果异常: %+v", page)
	}
	// 页脚统计: 来自聚合查询(未出现的状态补 0)
	if page.Counts[StatusRunning] != 3 || page.Counts[StatusError] != 1 || page.Counts[StatusStopped] != 0 {
		t.Fatalf("状态统计异常: %+v", page.Counts)
	}
	// 关键契约: 读必须走 CmdQuery —— 引擎里只有它做 sqlx.Select 填充 Into;
	// 用 CmdGet 会拿到通用行集、Into 恒为空(曾因此导致列表空、无报错)。
	store.mu.Lock()
	cmds := append([]domainstore.Cmd{}, store.cmds...)
	store.mu.Unlock()
	for _, cmd := range cmds {
		if cmd != domainstore.CmdQuery {
			t.Fatalf("读路径只应使用 CmdQuery, got %v", cmd)
		}
	}

	query := store.lastQuery()
	for _, fragment := range []string{"deleted_at = 0", "status = ?", "name LIKE ? OR target LIKE ?", "LIMIT ? OFFSET ?"} {
		if !strings.Contains(query.sql, fragment) {
			t.Fatalf("查询缺少 %q: %q", fragment, query.sql)
		}
	}
	// 参数顺序: workspace(收束) -> status, like, like, pageSize, offset
	if fmt.Sprint(query.args) != fmt.Sprint([]any{"w-test", StatusRunning, "%local%", "%local%", 5, 10}) {
		t.Fatalf("查询参数异常: %v", query.args)
	}
}

func TestListEmptySkipsRowQuery(t *testing.T) {
	svc, store, _ := newService()
	store.total = 0

	page, err := svc.List(nil, Query{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("空结果应返回空列表: %+v", page)
	}
	// 只做 COUNT 与状态聚合(两次), 不再查列表页
	for _, item := range store.queries {
		if strings.Contains(item.sql, "LIMIT ? OFFSET ?") {
			t.Fatalf("总数为 0 时不应查列表页: %q", item.sql)
		}
	}
	if len(store.queries) != 2 {
		t.Fatalf("应只有 COUNT 与状态聚合两次查询, got %d", len(store.queries))
	}
}

// 回归: 带用户过滤时 WHERE 参数顺序必须与占位符一致, 且影响行数为 0 时不能报成功。
func TestUpdateRespectsUserScopeOrder(t *testing.T) {
	svc, store, _ := newService()
	svc.currentUser = func() string { return "u1" }
	store.row = &Proxy{ID: "p1", Name: "old", Target: "t", Status: StatusStopped, Rules: "{}"}

	if _, err := svc.Update(nil, "p1", Input{Name: "new", Target: "t", Rules: "{}"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	exec := store.lastExec()
	if !strings.Contains(exec.sql, "user_id = ?") {
		t.Fatalf("应带用户过滤: %q", exec.sql)
	}
	// SET 5 个参数 + WHERE(id, user_id)
	if len(exec.args) != 7 || exec.args[5] != "p1" || exec.args[6] != "u1" {
		t.Fatalf("参数顺序应与占位符一致, got %#v", exec.args)
	}

	store.affected = 0
	if _, err := svc.Update(nil, "p1", Input{Name: "new2", Target: "t", Rules: "{}"}); err == nil {
		t.Fatalf("影响行数为 0 时应返回 not found")
	}
}

// rules 采用最新模板格式(host/port/uiport/timeout/dnsServer): 越界与非法 JSON 直接拒绝。
func TestCreateRejectsInvalidRules(t *testing.T) {
	svc, _, _ := newService()

	cases := []string{
		`{"port": 70000}`,
		`{"uiport": 70000}`,
		`{"timeout": -1}`,
		`not-json`,
	}
	for _, rules := range cases {
		if _, err := svc.Create(nil, Input{Name: "p", Rules: rules}); err == nil {
			t.Fatalf("rules=%q 应被拒绝", rules)
		}
	}

	if _, err := svc.Create(nil, Input{Name: "ok", Rules: `{"host":"127.0.0.1","port":8899,"uiport":8899,"timeout":30000,"dnsServer":"1.1.1.1"}`}); err != nil {
		t.Fatalf("合法 rules 应通过: %v", err)
	}
}

// 删除代理时应一并清理实例运行目录(<proxyRoot>/<workspace>.<name>)。
func TestDeleteRemovesStorageDir(t *testing.T) {
	svc, store, _ := newService()
	svc.root = t.TempDir()
	svc.proxyRoot = svc.resolveBase("proxies")
	store.row = &Proxy{ID: "p1", Name: "local-dev", WorkspaceID: "w-test"}

	dir := svc.baseDirOf(svc.workspaceKey(*store.row), store.row.Name)
	if err := os.MkdirAll(filepath.Join(dir, "rule"), 0o755); err != nil {
		t.Fatalf("准备目录: %v", err)
	}

	if err := svc.Delete(nil, "p1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("storage 目录应被删除: %v", err)
	}
}

// 运行中的代理不允许删除。
func TestDeleteRejectsRunning(t *testing.T) {
	svc, store, _ := newService()
	store.row = &Proxy{ID: "p1", Name: "api", Status: StatusRunning}

	if err := svc.Delete(nil, "p1"); err == nil || !strings.Contains(err.Error(), "running") {
		t.Fatalf("运行中删除应被拒绝, got %v", err)
	}
}

// 创建同 workspace+name 时清除历史残留目录(此前删除未清理干净)。
func TestCreateClearsStaleStorage(t *testing.T) {
	svc, _, _ := newService()
	svc.root = t.TempDir()
	svc.proxyRoot = svc.resolveBase("proxies")

	stale := svc.baseDirOf("w-test", "api")
	if err := os.MkdirAll(filepath.Join(stale, ".whistle"), 0o755); err != nil {
		t.Fatalf("准备残留目录: %v", err)
	}

	if _, err := svc.Create(nil, Input{Name: "api", Rules: `{"host":"127.0.0.1","port":8899}`}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("残留目录应被清除: %v", err)
	}
}

// 批量删除: 能删的删掉, 运行中的跳过并汇总 warn(不中断其余删除)。
func TestDeleteManySkipsRunning(t *testing.T) {
	svc, store, logger := newService()
	stopped := &Proxy{ID: "p1", Name: "a", Status: StatusStopped}
	running := &Proxy{ID: "p2", Name: "b", Status: StatusRunning}

	// mustGet 依据 id 返回不同行
	store.byID = map[string]Proxy{"p1": *stopped, "p2": *running}

	result, err := svc.DeleteMany(nil, []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "p1" {
		t.Fatalf("应只删除 p1: %+v", result.Deleted)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].ID != "p2" || result.Skipped[0].Reason != "running" {
		t.Fatalf("p2 应因 running 跳过: %+v", result.Skipped)
	}
	logger.find(t, "proxy: bulk delete skipped")
}
