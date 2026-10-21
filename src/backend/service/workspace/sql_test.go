package workspace

import (
	stdctx "context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	domainstore "github.com/wensboy/automan/backend/domain/store"
	databaseengine "github.com/wensboy/automan/backend/engine/database"
)

// sqlStore 把 db 指令落到真实 sqlite(与启动链同源), 用于验证本服务生成的 SQL 真能执行。
// 这类测试专门拦 db tag / 列名不匹配(例: missing destination name created_at)。
type sqlStore struct{ db *sqlx.DB }

func (s *sqlStore) Submit(domainstore.Task) <-chan domainstore.Result {
	ch := make(chan domainstore.Result, 1)
	ch <- domainstore.Result{}
	close(ch)
	return ch
}

func (s *sqlStore) Do(_ stdctx.Context, task domainstore.Task) (any, error) {
	switch task.Cmd {
	case domainstore.CmdExec:
		args := task.Args.(domainstore.ExecArgs)
		result, err := s.db.Exec(args.SQL, args.Args...)
		if err != nil {
			return nil, err
		}
		affected, _ := result.RowsAffected()
		return domainstore.ExecResult{RowsAffected: affected}, nil
	case domainstore.CmdQuery:
		args := task.Args.(domainstore.QueryArgs)
		switch into := args.Into.(type) {
		case *[]Workspace:
			if err := s.db.Select(into, args.SQL, args.Args...); err != nil {
				return nil, err
			}
			return domainstore.QueryResult{Count: len(*into)}, nil
		case *[]int:
			if err := s.db.Select(into, args.SQL, args.Args...); err != nil {
				return nil, err
			}
			return domainstore.QueryResult{Count: len(*into)}, nil
		}
	}
	return nil, errors.New("sqlStore: unsupported cmd")
}

func newRealService(t *testing.T, user string) (*Service, *sqlx.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace.db")
	if err := databaseengine.Migrate(os.DirFS("../../../data"), "migrate", path); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	db, err := sqlx.Connect("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := New().WithStore(&sqlStore{db: db}).WithCurrentUser(func() string { return user })
	return svc, db
}

// 真实库闭环: 自动 Default -> 创建 -> 重名 -> 切换 -> 删除 -> 跨用户隔离。
func TestServiceAgainstRealSQLite(t *testing.T) {
	svc, db := newRealService(t, "u-1")

	// 列表为空且不自动创建
	items, err := svc.List(nil)
	if err != nil {
		t.Fatalf("List(空): %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("不应自动创建工作区, got %+v", items)
	}
	var rows int
	if err := db.Get(&rows, `SELECT COUNT(*) FROM workspaces WHERE user_id = 'u-1'`); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 0 {
		t.Fatalf("库中不应有行, got %d", rows)
	}

	created, err := svc.Create(nil, "Demo")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(nil, "Demo"); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("重名应被拒, got %v", err)
	}

	if err := svc.SetCurrent(nil, created.ID); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	if got := svc.Current(nil); got != created.ID {
		t.Fatalf("当前应为 %q, got %q", created.ID, got)
	}

	if err := svc.Delete(nil, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := svc.Current(nil); got != "" {
		t.Fatalf("删除当前工作区后应为空, got %q", got)
	}
	if err := db.Get(&rows, `SELECT COUNT(*) FROM workspaces WHERE id = ? AND deleted_at > 0`, created.ID); err != nil {
		t.Fatalf("count deleted: %v", err)
	}
	if rows != 1 {
		t.Fatalf("应为逻辑删除, got %d", rows)
	}

	// 跨用户隔离: u-2 看不到 u-1 的工作区, 且会自动得到自己的 Default
	other, _ := newRealServiceWithDB(t, db, "u-2")
	otherItems, err := other.List(nil)
	if err != nil {
		t.Fatalf("List(u-2): %v", err)
	}
	if len(otherItems) != 0 {
		t.Fatalf("u-2 不应看到 u-1 的工作区, got %+v", otherItems)
	}
}

func newRealServiceWithDB(t *testing.T, db *sqlx.DB, user string) (*Service, *sqlx.DB) {
	t.Helper()
	return New().WithStore(&sqlStore{db: db}).WithCurrentUser(func() string { return user }), db
}
