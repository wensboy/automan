package proxy

import (
	stdctx "context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	databaseengine "github.com/wensboy/automan/backend/engine/database"
)

// sqlStore 把 store 的 db 指令落到**真实 sqlite** 上: 用于验证 service 生成的 SQL 真能执行。
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
		lastID, _ := result.LastInsertId()
		return domainstore.ExecResult{RowsAffected: affected, LastInsertID: lastID}, nil

	case domainstore.CmdQuery:
		args := task.Args.(domainstore.QueryArgs)
		switch into := args.Into.(type) {
		case *[]int:
			if err := s.db.Select(into, args.SQL, args.Args...); err != nil {
				return nil, err
			}
			return domainstore.QueryResult{Count: len(*into)}, nil
		case *[]Proxy:
			if err := s.db.Select(into, args.SQL, args.Args...); err != nil {
				return nil, err
			}
			return domainstore.QueryResult{Count: len(*into)}, nil
		default: // 通用行集(状态聚合)
			rows, err := s.db.Queryx(args.SQL, args.Args...)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			generic := []domainstore.Row{}
			for rows.Next() {
				item := domainstore.Row{}
				if err := rows.MapScan(item); err != nil {
					return nil, err
				}
				generic = append(generic, item)
			}
			return domainstore.QueryResult{Count: len(generic), Rows: generic}, nil
		}
	}
	return nil, errors.New("sqlStore: unsupported cmd")
}

// 建表: 复用仓库里的迁移(与启动链同一条路径)。
func newSQLStore(t *testing.T) *sqlStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "proxy.db")
	if err := migrateForTest(path); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	db, err := sqlx.Connect("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &sqlStore{db: db}
}

// 真实链路上的 CRUD: 建 → 查 → 改 → 搜 → 统计 → 逻辑删除 → 同名重建。
func TestServiceAgainstRealSQLite(t *testing.T) {
	store := newSQLStore(t)
	logger := &fakeLogger{}
	svc := New(store, logger)

	created, err := svc.Create(nil, Input{Name: "local-dev", Target: "127.0.0.1:8899", Rules: `{"port":8899}`})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" || created.CreatedAt == 0 || created.Status != StatusStopped {
		t.Fatalf("创建结果异常: %+v", created)
	}

	// 重复名被拒(应用层)
	if _, err := svc.Create(nil, Input{Name: "local-dev", Target: "t"}); err == nil ||
		!strings.Contains(err.Error(), "duplicate name") {
		t.Fatalf("重复名应被拒绝, got %v", err)
	}

	second, err := svc.Create(nil, Input{Name: "stage-api", Target: "10.0.20.5:8080", Status: StatusRunning})
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	// 列表 + 搜索 + 状态统计
	page, err := svc.List(nil, Query{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("列表应有 2 条, got total=%d items=%d", page.Total, len(page.Items))
	}
	if page.Counts[StatusRunning] != 1 || page.Counts[StatusStopped] != 1 {
		t.Fatalf("状态统计异常: %+v", page.Counts)
	}

	filtered, err := svc.List(nil, Query{Search: "stage", Status: StatusRunning})
	if err != nil {
		t.Fatalf("List filtered: %v", err)
	}
	if filtered.Total != 1 || filtered.Items[0].Name != "stage-api" {
		t.Fatalf("搜索+状态过滤异常: %+v", filtered)
	}

	paged, err := svc.List(nil, Query{Page: 2, PageSize: 1})
	if err != nil {
		t.Fatalf("List paged: %v", err)
	}
	if paged.Total != 2 || len(paged.Items) != 1 {
		t.Fatalf("分页异常: %+v", paged)
	}

	// 更新: 改状态与规则, 更新时间应当推进
	updated, err := svc.Update(nil, created.ID, Input{
		Name: "local-dev", Target: "127.0.0.1:8899", Status: StatusRunning, Rules: `{"port":9000}`,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Status != StatusRunning || updated.Rules != `{"port":9000}` {
		t.Fatalf("更新结果异常: %+v", updated)
	}

	// 逻辑删除: 列表里消失, 同名可以重建
	if err := svc.Delete(nil, second.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	after, err := svc.List(nil, Query{})
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if after.Total != 1 {
		t.Fatalf("删除后应剩 1 条, got %d", after.Total)
	}
	if _, err := svc.Get(nil, second.ID); err == nil {
		t.Fatalf("已删除的行不应可读")
	}
	recreated, err := svc.Create(nil, Input{Name: "stage-api", Target: "10.0.20.6:8080"})
	if err != nil {
		t.Fatalf("逻辑删除后同名应可重建: %v", err)
	}

	// 批量改状态(动态 IN 占位符 + 只改状态不同的行)
	changed, err := svc.SetStatus(nil, []string{created.ID, recreated.ID}, StatusRunning)
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if changed != 1 { // created 已是 running, recreated 是 stopped
		t.Fatalf("应只改动 1 条(状态不同的那条), got %d", changed)
	}
	again, err := svc.SetStatus(nil, []string{recreated.ID}, StatusRunning)
	if err != nil {
		t.Fatalf("SetStatus 重复调用: %v", err)
	}
	if again != 0 {
		t.Fatalf("已处于目标状态不应再改, got %d", again)
	}

	// 状态统计不受状态过滤影响: 过滤 running 时仍能看到 stopped 的计数
	byStatus, err := svc.List(nil, Query{Status: StatusRunning})
	if err != nil {
		t.Fatalf("List by status: %v", err)
	}
	if byStatus.Total != 2 {
		t.Fatalf("按 running 过滤应命中 2 条, got %d", byStatus.Total)
	}
	if byStatus.Counts[StatusRunning] != 2 || byStatus.Counts[StatusStopped] != 0 {
		t.Fatalf("统计口径异常: %+v", byStatus.Counts)
	}

	// 长列表下的动态占位符(多条 id)
	ids := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		row, err := svc.Create(nil, Input{Name: fmt.Sprintf("bulk-%d", i), Target: "127.0.0.1:1"})
		if err != nil {
			t.Fatalf("Create bulk %d: %v", i, err)
		}
		ids = append(ids, row.ID)
	}
	bulk, err := svc.SetStatus(nil, ids, StatusException)
	if err != nil {
		t.Fatalf("SetStatus bulk: %v", err)
	}
	if bulk != 8 {
		t.Fatalf("批量应改动 8 条, got %d", bulk)
	}
	exceptions, err := svc.List(nil, Query{Status: StatusException})
	if err != nil {
		t.Fatalf("List exceptions: %v", err)
	}
	if exceptions.Total != 8 {
		t.Fatalf("应有 8 条 exception, got %d", exceptions.Total)
	}
}

// 迁移与启动链同源: 直接复用 data/migrate(测试工作目录在包内, 用相对路径回到 src/data)。
func migrateForTest(path string) error {
	return databaseengine.Migrate(os.DirFS("../../../data"), "migrate", path)
}

var _ = sql.ErrNoRows
var _ domainlog.Logger = (*fakeLogger)(nil)
