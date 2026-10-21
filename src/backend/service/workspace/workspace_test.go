package workspace

import (
	stdctx "context"
	"strings"
	"sync"
	"testing"

	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	domainstore "github.com/wensboy/automan/backend/domain/store"
)

// fakeStore 只实现本服务用到的 Exec/Query(与 proxy 测试同思路: 记录 SQL, 按需返回计数).
type fakeStore struct {
	mu    sync.Mutex
	execs []string
	// count: 通用计数(数量上限用); nameCount: 重名检查用
	count     int
	nameCount int
	rows      []Workspace
	affected  int64
}

func (f *fakeStore) Submit(domainstore.Task) <-chan domainstore.Result { panic("unused") }

func (f *fakeStore) Do(_ stdctx.Context, task domainstore.Task) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch task.Cmd {
	case domainstore.CmdExec:
		args := task.Args.(domainstore.ExecArgs)
		f.execs = append(f.execs, args.SQL)
		if f.affected == 0 {
			f.affected = 1
		}
		return domainstore.ExecResult{RowsAffected: f.affected}, nil
	case domainstore.CmdQuery:
		args := task.Args.(domainstore.QueryArgs)
		if strings.Contains(args.SQL, "COUNT(*)") {
			value := f.count
			if strings.Contains(args.SQL, "name = ?") {
				value = f.nameCount
			}
			if into, ok := args.Into.(*[]int); ok {
				*into = []int{value}
			}
			return domainstore.QueryResult{Count: 1}, nil
		}
		if into, ok := args.Into.(*[]Workspace); ok {
			*into = append([]Workspace{}, f.rows...)
		}
		return domainstore.QueryResult{Count: len(f.rows)}, nil
	}
	return nil, nil
}

func newService(user string) (*Service, *fakeStore) {
	store := &fakeStore{}
	svc := New().WithStore(store).WithCurrentUser(func() string { return user })
	return svc, store
}

// 创建: 插入带 user_id; 同用户内重名被拒.
func TestCreateWorkspace(t *testing.T) {
	svc, store := newService("u-1")
	created, err := svc.Create(nil, "  Demo  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "Demo" || created.ID == "" {
		t.Fatalf("结果异常: %+v", created)
	}
	if len(store.execs) != 1 || !strings.Contains(store.execs[0], "INSERT INTO workspaces") {
		t.Fatalf("应写入 workspaces, got %+v", store.execs)
	}

	store.nameCount = 1
	if _, err := svc.Create(nil, "Demo"); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("重名应被拒, got %v", err)
	}
}

// 列表为空时**不**自动创建工作区(v21 §10): 由用户显式创建。
func TestListDoesNotCreateDefault(t *testing.T) {
	svc, store := newService("u-1")
	items, err := svc.List(nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("不应自动创建, got %+v", items)
	}
	if got := svc.Current(nil); got != "" {
		t.Fatalf("未选中时当前工作区应为空, got %q", got)
	}
	if len(store.execs) != 0 {
		t.Fatalf("不应写库, got %+v", store.execs)
	}
}

// 切换与删除: 删除当前工作区会清空当前.
func TestSwitchAndDelete(t *testing.T) {
	svc, store := newService("u-1")
	// fake store 不解析 SQL: 仅让 find() 命中目标行
	store.rows = []Workspace{{ID: "w-2", Name: "B"}}

	if err := svc.SetCurrent(nil, "w-2"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	if got := svc.Current(nil); got != "w-2" {
		t.Fatalf("当前应为 w-2, got %q", got)
	}

	if err := svc.Delete(nil, "w-2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := svc.Current(nil); got != "" {
		t.Fatalf("删除当前工作区后应为空, got %q", got)
	}
}

// 未登录: 明确报错, 不产生任何 SQL.
func TestRequiresUser(t *testing.T) {
	svc, store := newService("")
	if _, err := svc.List(nil); err == nil || !strings.Contains(err.Error(), "signed-in") {
		t.Fatalf("未登录应报错, got %v", err)
	}
	if len(store.execs) != 0 {
		t.Fatalf("未登录不应写库, got %+v", store.execs)
	}
}

// fakeSettings 只实现 Get(本服务只读取上限).
type fakeSettings struct{ values map[string]any }

func (f fakeSettings) Get(name string) (any, bool) {
	value, ok := f.values[name]
	return value, ok
}
func (f fakeSettings) Set(string, any) error           { return nil }
func (f fakeSettings) Save() error                     { return nil }
func (f fakeSettings) Reload() error                   { return nil }
func (f fakeSettings) Reset(string) error              { return nil }
func (f fakeSettings) View(string) domainsettings.View { return domainsettings.ViewNone }

// 数量上限来自 general.workspace.limit: 达到上限后创建被拒。
func TestCreateRespectsLimit(t *testing.T) {
	svc, store := newService("u-1")
	svc.settings = fakeSettings{values: map[string]any{keyLimit: 2}}
	store.count = 0
	store.nameCount = 0

	if _, err := svc.Create(nil, "A"); err != nil {
		t.Fatalf("Create(A): %v", err)
	}
	store.count = 1
	if _, err := svc.Create(nil, "B"); err != nil {
		t.Fatalf("Create(B): %v", err)
	}

	store.count = 2 // 已达上限
	if _, err := svc.Create(nil, "C"); err == nil || !strings.Contains(err.Error(), "limit reached") {
		t.Fatalf("超上限应被拒, got %v", err)
	}
}
