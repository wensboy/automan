package session

import (
	stdctx "context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"

	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	databaseengine "github.com/wensboy/automan/backend/engine/database"
)

// sqlStore 把 db 指令落到**真实 sqlite** 上(迁移与启动链同源), 用于验证 session/user 生成的 SQL 真能执行。
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
		case *[]userRow:
			if err := s.db.Select(into, args.SQL, args.Args...); err != nil {
				return nil, err
			}
			return domainstore.QueryResult{Count: len(*into)}, nil
		case *[]loginRow:
			if err := s.db.Select(into, args.SQL, args.Args...); err != nil {
				return nil, err
			}
			return domainstore.QueryResult{Count: len(*into)}, nil
		}
	}
	return nil, errors.New("sqlStore: unsupported cmd")
}

// 真实链路上的闭环: 注册 -> 登录失败/成功 -> 改密 -> 登出 -> 重启恢复。
func TestServiceAgainstRealSQLite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.db")
	if err := databaseengine.Migrate(os.DirFS("../../../data"), "migrate", path); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	db, err := sqlx.Connect("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	h := newHarness(t) // 复用测试 harness 里的错误码注册与时钟
	h.svc.store = &sqlStore{db: db}
	h.svc.cache.Delete("") // 清空(空 key 无副作用)

	// 注册 -> 落库(users + login_logs)
	session, err := h.svc.SignUp(nil, SignUpInput{Username: "alice", Password: "password123", Nickname: "Alice"})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if session.Token == "" || session.User.Nickname != "Alice" {
		t.Fatalf("会话异常: %+v", session)
	}
	var userCount, logCount int
	if err := db.Get(&userCount, `SELECT COUNT(*) FROM users WHERE username = 'alice' AND deleted_at = 0`); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if err := db.Get(&logCount, `SELECT COUNT(*) FROM login_logs WHERE username = 'alice' AND result = 'success'`); err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if userCount != 1 || logCount != 1 {
		t.Fatalf("应各写 1 行, got users=%d logs=%d", userCount, logCount)
	}

	// 口令以 bcrypt 落库, 不是明文
	var stored string
	if err := db.Get(&stored, `SELECT password FROM users WHERE username = 'alice'`); err != nil {
		t.Fatalf("read password: %v", err)
	}
	if stored == "password123" || len(stored) < 20 {
		t.Fatalf("口令应哈希落库, got %q", stored)
	}

	// 登录失败 -> failed 日志; 成功 -> 新会话
	if _, err := h.svc.SignIn(nil, "alice", "bad-password"); err == nil {
		t.Fatalf("错误口令应失败")
	}
	if err := db.Get(&logCount, `SELECT COUNT(*) FROM login_logs WHERE username = 'alice' AND result = 'failed'`); err != nil {
		t.Fatalf("count failed logs: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("应有 1 条 failed 日志, got %d", logCount)
	}

	second, err := h.svc.SignIn(nil, "alice", "password123")
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	// 模拟重启: 清缓存后凭 token 从 login_logs 恢复
	h.svc.cache.Delete(second.Token)
	restored, err := h.svc.Resume(nil, second.Token)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if restored.User.Username != "alice" || restored.User.Nickname != "Alice" {
		t.Fatalf("恢复的用户异常: %+v", restored.User)
	}

	// 改密 -> 旧会话失效 + 新口令可登录
	if err := h.svc.ChangePassword(nil, second.Token, "password123", "newpassword1"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := h.svc.Current(nil, second.Token); err == nil {
		t.Fatalf("改密后旧会话应失效")
	}
	if _, err := h.svc.SignIn(nil, "alice", "newpassword1"); err != nil {
		t.Fatalf("新口令应能登录: %v", err)
	}

	// 登出: 缓存清空后 Current 失败
	final, err := h.svc.SignIn(nil, "alice", "newpassword1")
	if err != nil {
		t.Fatalf("SignIn final: %v", err)
	}
	if err := h.svc.SignOut(nil, final.Token); err != nil {
		t.Fatalf("SignOut: %v", err)
	}
	if _, err := h.svc.Current(nil, final.Token); err == nil {
		t.Fatalf("登出后会话应失效")
	}
}

var _ domainsettings.Store = (*fakeSettings)(nil)
var _ domainlog.Logger = nil
