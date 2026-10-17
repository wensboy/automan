package session

import (
	stdctx "context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wensboy/automan/backend/domain/event"
	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	apperr "github.com/wensboy/automan/backend/internal/err"
)

// ---- 测试替身 ----

type fakeStore struct {
	mu     sync.Mutex
	users  []map[string]any
	logins []map[string]any
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

	switch task.Cmd {
	case domainstore.CmdExec:
		args := task.Args.(domainstore.ExecArgs)
		switch {
		case strings.Contains(args.SQL, "INSERT INTO users"):
			f.users = append(f.users, map[string]any{
				"id": args.Args[0], "username": args.Args[1], "password": args.Args[2],
				"nickname": args.Args[3], "created_at": args.Args[4], "updated_at": args.Args[5],
				"deleted_at": int64(0),
			})
		case strings.Contains(args.SQL, "INSERT INTO login_logs"):
			f.logins = append(f.logins, map[string]any{
				"id": args.Args[0], "user_id": args.Args[1], "username": args.Args[2],
				"result": args.Args[3], "reason": args.Args[4], "token_hash": args.Args[5], "created_at": args.Args[6],
			})
		case strings.Contains(args.SQL, "UPDATE users SET password ="):
			for _, row := range f.users {
				if row["id"] == args.Args[2] {
					row["password"] = args.Args[0]
				}
			}
		}
		return domainstore.ExecResult{RowsAffected: 1}, nil

	case domainstore.CmdQuery:
		args := task.Args.(domainstore.QueryArgs)
		switch {
		case strings.Contains(args.SQL, "FROM users"):
			into, _ := args.Into.(*[]userRow)
			for _, row := range f.users {
				if args.Args[0] != row["username"] && args.Args[0] != row["id"] {
					continue
				}
				if into != nil {
					*into = append(*into, rowToUser(row))
				}
			}
			return domainstore.QueryResult{Count: 1}, nil

		case strings.Contains(args.SQL, "FROM login_logs"):
			into, _ := args.Into.(*[]loginRow)
			for _, row := range f.logins {
				if row["token_hash"] != args.Args[0] || row["result"] != "success" {
					continue
				}
				if into != nil {
					*into = append(*into, loginRow{
						ID: row["id"].(string), UserID: row["user_id"].(string), Username: row["username"].(string),
						Result: row["result"].(string), Reason: row["reason"].(string),
						TokenHash: row["token_hash"].(string), CreatedAt: row["created_at"].(int64),
					})
				}
			}
			return domainstore.QueryResult{Count: 1}, nil
		}
	}
	return domainstore.QueryResult{}, nil
}

func rowToUser(row map[string]any) userRow {
	asInt := func(v any) int64 {
		if n, ok := v.(int64); ok {
			return n
		}
		return 0
	}
	asString := func(v any) string {
		if s, ok := v.(string); ok {
			return s
		}
		return ""
	}
	return userRow{
		ID: asString(row["id"]), Username: asString(row["username"]),
		Password: asString(row["password"]), Nickname: asString(row["nickname"]),
		CreatedAt: asInt(row["created_at"]), UpdatedAt: asInt(row["updated_at"]),
	}
}

// fakeSettings 模拟 settings engine 的 Get(只读生效值)。
type fakeSettings struct{ values map[string]any }

func (f *fakeSettings) Get(name string) (any, bool) {
	v, ok := f.values[name]
	return v, ok
}
func (f *fakeSettings) Set(string, any) error           { return nil }
func (f *fakeSettings) Save() error                     { return nil }
func (f *fakeSettings) Reload() error                   { return nil }
func (f *fakeSettings) Reset(string) error              { return nil }
func (f *fakeSettings) View(string) domainsettings.View { return domainsettings.ViewUser }

// fakeBus 记录发出的主题。
type fakeBus struct {
	mu     sync.Mutex
	topics []event.Topic
}

func (f *fakeBus) Emit(topic event.Topic, _ any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.topics = append(f.topics, topic)
}
func (f *fakeBus) Subscribe(event.Topic, func(any)) func() { return func() {} }

func (f *fakeBus) count(topic event.Topic) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, t := range f.topics {
		if t == topic {
			n++
		}
	}
	return n
}

type harness struct {
	svc      *Service
	store    *fakeStore
	settings *fakeSettings
	bus      *fakeBus
	clock    time.Time
}

var errOnce sync.Once

func newHarness(t *testing.T) *harness {
	t.Helper()
	// 错误码在真实链路里由 data/conf/error.json 注册; 测试里补一次(apperr.New 需要已知码)
	errOnce.Do(func() {
		_ = apperr.Register([]apperr.Entry{
			{Code: apperr.CodeNotFound, Msg: "not found"},
			{Code: apperr.CodeServiceFail, Msg: "service fail"},
		})
	})
	store := &fakeStore{}
	settings := &fakeSettings{values: map[string]any{
		keyExpirationS:   3600,
		keyResumeOnStart: true,
		keySlidingExpire: true,
	}}
	bus := &fakeBus{}
	h := &harness{store: store, settings: settings, bus: bus, clock: time.UnixMilli(1_700_000_000_000)}
	svc := New(store, settings, bus, nil)
	svc.now = func() time.Time { return h.clock }
	svc.newID = func() string { return "id-" + time.Now().Format("150405.000000") }
	h.svc = svc
	t.Cleanup(svc.Close)
	return h
}

func (h *harness) advance(d time.Duration) { h.clock = h.clock.Add(d) }

// ---- 用例 ----

func TestSignUpCreatesUserAndSession(t *testing.T) {
	h := newHarness(t)
	got, err := h.svc.SignUp(nil, SignUpInput{Username: "alice", Password: "password123", Nickname: "Alice"})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if got.Token == "" || got.User.Username != "alice" || got.ExpiresAt == 0 {
		t.Fatalf("会话异常: %+v", got)
	}
	if len(h.store.users) != 1 || len(h.store.logins) != 1 {
		t.Fatalf("应写 1 用户 + 1 登录日志, got %d/%d", len(h.store.users), len(h.store.logins))
	}
	if h.bus.count(event.TopicSessionSignup) != 1 {
		t.Fatalf("应发 session.signup 事件")
	}
	// 口令必须是哈希(不等于明文)
	if h.store.users[0]["password"] == "password123" {
		t.Fatalf("口令不应明文落库")
	}
}

func TestSignUpRejectsDuplicate(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.SignUp(nil, SignUpInput{Username: "bob", Password: "password123"}); err != nil {
		t.Fatalf("首次注册: %v", err)
	}
	if _, err := h.svc.SignUp(nil, SignUpInput{Username: "bob", Password: "password123"}); err == nil ||
		!strings.Contains(err.Error(), "duplicate username") {
		t.Fatalf("重名应被拒绝, got %v", err)
	}
}

func TestSignInRecordsFailureAndSuccess(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.SignUp(nil, SignUpInput{Username: "carol", Password: "password123"}); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if _, err := h.svc.SignIn(nil, "carol", "wrong-password"); err == nil {
		t.Fatalf("错误口令应被拒绝")
	}
	if h.store.logins[len(h.store.logins)-1]["result"] != "failed" {
		t.Fatalf("应记一条 failed 登录日志")
	}

	session, err := h.svc.SignIn(nil, "carol", "password123")
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if session.Token == "" || h.bus.count(event.TopicSessionLogin) != 1 {
		t.Fatalf("登录应成功并发事件")
	}
	if _, err := h.svc.Current(nil, session.Token); err != nil {
		t.Fatalf("Current: %v", err)
	}
}

func TestSignOutInvalidatesToken(t *testing.T) {
	h := newHarness(t)
	session, err := h.svc.SignUp(nil, SignUpInput{Username: "dave", Password: "password123"})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := h.svc.SignOut(nil, session.Token); err != nil {
		t.Fatalf("SignOut: %v", err)
	}
	if _, err := h.svc.Current(nil, session.Token); err == nil {
		t.Fatalf("登出后会话应失效")
	}
	if h.bus.count(event.TopicSessionLogout) != 1 {
		t.Fatalf("应发 session.logout 事件")
	}
}

func TestResumeWithinAndBeyondExpiration(t *testing.T) {
	h := newHarness(t)
	session, err := h.svc.SignUp(nil, SignUpInput{Username: "erin", Password: "password123"})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	// 模拟重启: 清空缓存, 只在 login_logs 里留下 token 哈希
	h.svc.cache.Delete(session.Token)
	if _, err := h.svc.Current(nil, session.Token); err == nil {
		t.Fatalf("缓存清空后 Current 应失败")
	}

	// 有效期内 -> 恢复成功
	if _, err := h.svc.Resume(nil, session.Token); err != nil {
		t.Fatalf("有效期内应能恢复: %v", err)
	}

	// 超过有效期 -> 恢复失败
	h.svc.cache.Delete(session.Token)
	h.advance(2 * time.Hour) // expirationS = 3600
	if _, err := h.svc.Resume(nil, session.Token); err == nil ||
		!strings.Contains(err.Error(), "expired") {
		t.Fatalf("超过有效期应拒绝恢复, got %v", err)
	}
}

func TestChangePasswordInvalidatesSessions(t *testing.T) {
	h := newHarness(t)
	session, err := h.svc.SignUp(nil, SignUpInput{Username: "frank", Password: "password123"})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := h.svc.ChangePassword(nil, session.Token, "wrong", "newpassword1"); err == nil {
		t.Fatalf("旧口令错误应被拒绝")
	}
	if err := h.svc.ChangePassword(nil, session.Token, "password123", "newpassword1"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := h.svc.Current(nil, session.Token); err == nil {
		t.Fatalf("改密后旧会话应失效")
	}
	if _, err := h.svc.SignIn(nil, "frank", "newpassword1"); err != nil {
		t.Fatalf("新口令应能登录: %v", err)
	}
}

func TestSessionTTLCappedAtSevenDays(t *testing.T) {
	h := newHarness(t)
	h.settings.values[keyExpirationS] = int(30 * 24 * 3600) // 30 天 > 7 天上限
	h.svc.refreshConfig()                                   // 配置在构造时缓存, 直接改值后需刷新(真实链路由 ebus 触发)
	session, err := h.svc.SignUp(nil, SignUpInput{Username: "gina", Password: "password123"})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	ttl := time.UnixMilli(session.ExpiresAt).Sub(h.clock)
	if ttl > absoluteCap {
		t.Fatalf("TTL 应被 7 天上限收敛, got %v", ttl)
	}
}

// 设置变更经 ebus 通知后: 刷新配置并**回溯**应用到在线会话(受 7 天绝对上限约束)。
func TestSettingsChangeRetunesLiveSessions(t *testing.T) {
	h := newHarness(t)
	session, err := h.svc.SignUp(nil, SignUpInput{Username: "zoe", Password: "password123"})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	before := time.UnixMilli(session.ExpiresAt).Sub(h.clock)

	// 改成 30 天(超过绝对上限) -> 应被 7 天收敛, 且比原来更长
	h.settings.values[keyExpirationS] = int(30 * 24 * 3600)
	h.svc.onSettingsChanged(event.SettingsChanged{Key: keyExpirationS})

	item := h.svc.cache.Get(session.Token)
	if item == nil {
		t.Fatalf("会话不应被清掉")
	}
	after := item.Value().ExpiresAt.Sub(h.clock)
	if after <= before {
		t.Fatalf("在线会话应被续期: before=%v after=%v", before, after)
	}
	if after > absoluteCap {
		t.Fatalf("应受 7 天上限约束, got %v", after)
	}

	// 无关键的变更不应触发续期
	h.settings.values[keyExpirationS] = 60
	h.svc.onSettingsChanged(event.SettingsChanged{Key: "general.language"})
	if item := h.svc.cache.Get(session.Token); item.Value().ExpiresAt.Sub(h.clock) != after {
		t.Fatalf("无关键不应改动会话")
	}
}
