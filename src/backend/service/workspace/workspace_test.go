package workspace

import (
	stdctx "context"
	"strings"
	"sync"
	"testing"

	domainstore "github.com/wensboy/automan/backend/domain/store"
)

// fakeStore 只实现本服务用到的 Exec/Query(与 proxy 测试同思路: 记录 SQL, 按需返回计数).
type fakeStore struct {
	mu       sync.Mutex
	execs    []string
	count    int
	rows     []Workspace
	affected int64
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
			if into, ok := args.Into.(*[]int); ok {
				*into = []int{f.count}
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
	svc := New(store, nil).WithCurrentUser(func() string { return user })
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

	store.count = 1
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
