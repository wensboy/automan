// Package session 编排用户会话: 注册 / 登录 / 登出 / 校验 / 启动恢复 / 改密.
//
// 依赖边界(见 docs/design/v18.md):
//   - **不新增 engine**: 只作为 srv 存在; 直接经 db store 执行 SQL、经 settings engine 读生效配置
//     (srv 之间不互相调用, 见 §4 的 [!notice]);
//   - 会话状态放内存 ttlcache: 到期自动清理, 每次校验可滑动续期, 过期钩子发事件;
//   - 启动恢复: 用 login_logs 里该 token 哈希对应的最后一条成功登录时间 + 有效期 - now > 0 判断;
//   - 用户数据由 service/user 负责 CRUD, 本包只读取(不调用它的服务)。
package session

import (
	stdctx "context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/authkit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
	serviceuser "github.com/wensboy/automan/backend/service/user"
)

// Key 是会话服务在 srvs 中的键.
var Key = key.New[*Service]("service.session")

const (
	// absoluteCap 是会话的绝对上限: 无论怎么续期, 单个会话最多存活 7 天(v18 §11 F).
	absoluteCap = 7 * 24 * time.Hour
	// fallbackExpirationS 是配置缺失时的兜底有效期(默认层有该键, 这里只防御性兜底).
	fallbackExpirationS = 7200

	// 配置键(与 data/default/settings.user.json 对齐).
	keyExpirationS   = "general.session.expirationS"
	keyResumeOnStart = "general.session.resumeOnStart"
	keySlidingExpire = "general.session.slidingExpire"

	minUsernameLen = 3
	maxUsernameLen = 32
	maxNicknameLen = 64
)

// SignUpInput 是注册入参.
type SignUpInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

// Session 是对外形态: 令牌 + 用户 + 过期时间(毫秒).
type Session struct {
	Token     string           `json:"token"`
	User      serviceuser.User `json:"user"`
	ExpiresAt int64            `json:"expiresAt"`
	LoginAt   int64            `json:"loginAt"` // 本次会话的登录时刻(unix 毫秒)
}

// entry 是缓存里的会话条目.
type entry struct {
	UserID    string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// userRow 是读取用户时用的内部行(含口令哈希).
type userRow struct {
	ID        string `db:"id"`
	Username  string `db:"username"`
	Password  string `db:"password"`
	Nickname  string `db:"nickname"`
	CreatedAt int64  `db:"created_at"`
	UpdatedAt int64  `db:"updated_at"`
}

func (r userRow) public() serviceuser.User {
	return serviceuser.User{
		ID: r.ID, Username: r.Username, Nickname: r.Nickname,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// loginRow 是 login_logs 的一行.
type loginRow struct {
	ID        string `db:"id"`
	UserID    string `db:"user_id"`
	Username  string `db:"username"`
	Result    string `db:"result"`
	Reason    string `db:"reason"`
	TokenHash string `db:"token_hash"`
	CreatedAt int64  `db:"created_at"`
}

// config 是运行期生效的会话配置: 经 ebus 订阅 settings 变更后刷新(服务不轮询设置).
type config struct {
	expiration    time.Duration
	resumeOnStart bool
	slidingExpire bool
}

// Service 编排会话; 依赖在构造时注入.
type Service struct {
	store    domainstore.Store
	settings domainsettings.Store
	ebus     event.Bus
	logger   domainlog.Logger
	cache    *ttlcache.Cache[string, entry]
	now      func() time.Time
	newID    func() string
	cfg      config
	cfgMu    sync.RWMutex
	unsub    []func()

	// currentUser 是"最近一次有效会话"的用户 id: 供其它服务(如 proxy)做数据收束。
	// 只保存在内存里, 不落库 —— 重启后由 Resume 重新确认。
	currentUser string
	userMu      sync.RWMutex
}

// New 创建会话服务并启动缓存清理.
// New 创建会话服务: 只做基础初始化(缓存 + 清理协程); 外部依赖经 WithXxx 链式注入。
func New() *Service {
	s := &Service{
		cache: ttlcache.New[string, entry](ttlcache.WithDisableTouchOnHit[string, entry]()),
		now:   time.Now,
		newID: newID,
	}
	s.cache.OnEviction(s.onEvicted)
	go s.cache.Start()
	return s
}

// WithStore 注入数据库 store.
func (s *Service) WithStore(store domainstore.Store) *Service { s.store = store; return s }

// WithSettings 注入设置引擎(注入后立即刷新生效配置).
func (s *Service) WithSettings(settings domainsettings.Store) *Service {
	s.settings = settings
	s.refreshConfig()
	return s
}

// WithEventBus 注入事件总线并订阅设置变更(影响后端的 key 走 ebus).
func (s *Service) WithEventBus(ebus event.Bus) *Service {
	s.ebus = ebus
	if ebus != nil {
		s.unsub = append(s.unsub,
			ebus.Subscribe(event.TopicSettingsChanged, s.onSettingsChanged),
			ebus.Subscribe(event.TopicSettingsReloaded, s.onSettingsReloaded),
		)
	}
	return s
}

// WithLogger 注入日志出口.
func (s *Service) WithLogger(logger domainlog.Logger) *Service { s.logger = logger; return s }

// Close 退订并停止清理协程(组合根在退出时调用).
func (s *Service) Close() {
	for _, off := range s.unsub {
		off()
	}
	s.unsub = nil
	s.cache.Stop()
}

// ---- 配置: 订阅 -> 刷新 -> 回溯应用到在线会话 ----

func (s *Service) get(name string) (any, bool) {
	if s.settings == nil {
		return nil, false
	}
	return s.settings.Get(name)
}

// refreshConfig 读取生效值(settings engine 的三视图叠加结果)并缓存.
func (s *Service) refreshConfig() {
	next := config{
		expiration:    time.Duration(fallbackExpirationS) * time.Second,
		resumeOnStart: true,
		slidingExpire: true,
	}
	if seconds := toInt2(s.get(keyExpirationS)); seconds > 0 {
		next.expiration = time.Duration(seconds) * time.Second
	}
	if value, ok := s.get(keyResumeOnStart); ok {
		if enabled, ok := value.(bool); ok {
			next.resumeOnStart = enabled
		}
	}
	if value, ok := s.get(keySlidingExpire); ok {
		if enabled, ok := value.(bool); ok {
			next.slidingExpire = enabled
		}
	}
	s.cfgMu.Lock()
	s.cfg = next
	s.cfgMu.Unlock()
}

func (s *Service) config() config {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg
}

// onSettingsChanged 只在会话相关键变化时刷新(其余键与本服务无关).
func (s *Service) onSettingsChanged(payload any) {
	changed, ok := cast.As[event.SettingsChanged](payload)
	if !ok {
		return
	}
	if changed.Key != keyExpirationS && changed.Key != keyResumeOnStart && changed.Key != keySlidingExpire {
		return
	}
	s.refreshConfig()
	s.retune()
}

// onSettingsReloaded 外部改文件后的重载: 载荷不带键列表, 整体刷新一次.
func (s *Service) onSettingsReloaded(payload any) {
	reloaded, ok := cast.As[event.SettingsReloaded](payload)
	if !ok || reloaded.Err != "" {
		return
	}
	s.refreshConfig()
	s.retune()
}

// retune 把新的有效期回溯应用到在线会话(仍受 7 天绝对上限约束).
func (s *Service) retune() {
	expiration := s.config().expiration
	now := s.now()
	for token, item := range s.cache.Items() {
		current := item.Value()
		ttl := s.capTTL(current.CreatedAt, expiration)
		if ttl <= 0 {
			s.cache.Delete(token)
			continue
		}
		next := entry{UserID: current.UserID, Username: current.Username,
			CreatedAt: current.CreatedAt, ExpiresAt: now.Add(ttl)}
		s.cache.Set(token, next, ttl)
	}
}

// enforceSingleUser 保证"同一时间只有一个有效登录用户":
// 新会话建立时, 把其它用户的会话全部作废(本机单用户使用场景, 避免旧用户数据继续可见)。
func (s *Service) enforceSingleUser(userID string) {
	trimmed := strings.TrimSpace(userID)
	if trimmed == "" {
		return
	}
	victims := make([]string, 0)
	s.cache.Range(func(item *ttlcache.Item[string, entry]) bool {
		if item.Value().UserID != trimmed {
			victims = append(victims, item.Key())
		}
		return true
	})
	for _, key := range victims {
		s.cache.Delete(key)
	}
	if len(victims) > 0 {
		s.info("session: other users invalidated", logkit.F("keep", trimmed), logkit.F("count", len(victims)))
	}
}

// CurrentUserID 返回最近一次有效会话的用户 id(未登录返回空串)。
// 供其它服务按用户收束数据(例: proxies 只看当前用户的代理), 不对外暴露 token。
func (s *Service) CurrentUserID() string {
	s.userMu.RLock()
	defer s.userMu.RUnlock()
	return s.currentUser
}

func (s *Service) setCurrentUser(userID string) {
	// 先作废其它用户的会话, 再记录当前用户(顺序不影响结果, 但语义上先清理)
	s.enforceSingleUser(userID)
	s.userMu.Lock()
	s.currentUser = strings.TrimSpace(userID)
	s.userMu.Unlock()
}

// clearCurrentUser 仅在退出的正是当前用户时清空(多会话下退掉其它会话不影响收束)。
func (s *Service) clearCurrentUser(userID string) {
	s.userMu.Lock()
	if s.currentUser == strings.TrimSpace(userID) {
		s.currentUser = ""
	}
	s.userMu.Unlock()
}

// SignUp 注册并立即登录.
func (s *Service) SignUp(_ domaincontext.Context, input SignUpInput) (Session, error) {
	username := strings.TrimSpace(input.Username)
	nickname := strings.TrimSpace(input.Nickname)
	switch {
	case len(username) < minUsernameLen || len(username) > maxUsernameLen:
		return Session{}, invalid(`username must be %d..%d chars`, minUsernameLen, maxUsernameLen)
	case !validUsername(username):
		return Session{}, invalid(`username allows letters, digits, "_" and "-" only`)
	case len(input.Password) < authkit.MinPasswordLen:
		return Session{}, invalid(`password must be at least %d chars`, authkit.MinPasswordLen)
	case len(nickname) > maxNicknameLen:
		return Session{}, invalid(`nickname exceeds %d chars`, maxNicknameLen)
	}

	if _, err := s.findUserByUsername(username); err == nil {
		return Session{}, invalid(`duplicate username %q`, username)
	} else if !isNotFound(err) {
		return Session{}, err
	}

	hash, err := authkit.HashPassword(input.Password)
	if err != nil {
		s.fail("signup", username, err)
		return Session{}, err
	}
	now := s.now().UnixMilli()
	if _, err := s.exec(`INSERT INTO users
        (id, username, password, nickname, created_at, updated_at, deleted_at)
        VALUES (?, ?, ?, ?, ?, ?, 0)`, s.newID(), username, hash, nickname, now, now); err != nil {
		s.fail("signup", username, err)
		return Session{}, err
	}

	row, err := s.findUserByUsername(username)
	if err != nil {
		return Session{}, err
	}
	result, err := s.openSession(row)
	if err != nil {
		return Session{}, err
	}
	s.setCurrentUser(row.ID)
	s.writeLoginLog(row.ID, username, "success", "", authkit.HashToken(result.Token))
	s.info("session: signup", logkit.F("id", row.ID), logkit.F("username", username))
	s.emit(event.TopicSessionSignup, event.SessionSignup{UserID: row.ID, Username: username, At: now})
	return result, nil
}

// SignIn 校验口令并建立会话.
func (s *Service) SignIn(_ domaincontext.Context, username, password string) (Session, error) {
	username = strings.TrimSpace(username)
	row, err := s.findUserByUsername(username)
	if err != nil {
		if isNotFound(err) {
			s.writeLoginLog("", username, "failed", "no such user", "")
			s.deliver(domainlog.LevelWarn, "session: login rejected",
				logkit.F("username", username), logkit.F("reason", "no such user"))
			return Session{}, invalid(`no such user %q`, username)
		}
		return Session{}, err
	}
	if !authkit.VerifyPassword(row.Password, password) {
		s.writeLoginLog(row.ID, username, "failed", "bad password", "")
		s.deliver(domainlog.LevelWarn, "session: login rejected",
			logkit.F("username", username), logkit.F("reason", "bad password"))
		return Session{}, invalid(`bad password for %q`, username)
	}

	result, err := s.openSession(row)
	if err != nil {
		return Session{}, err
	}
	s.setCurrentUser(row.ID)
	s.writeLoginLog(row.ID, username, "success", "", authkit.HashToken(result.Token))
	s.info("session: login", logkit.F("id", row.ID), logkit.F("username", username),
		logkit.F("expiresAt", result.ExpiresAt))
	s.emit(event.TopicSessionLogin, event.SessionLogin{UserID: row.ID, Username: username, At: s.now().UnixMilli()})
	return result, nil
}

// SignOut 结束会话.
func (s *Service) SignOut(_ domaincontext.Context, token string) error {
	item := s.cache.Get(strings.TrimSpace(token))
	if item == nil {
		return nil // 已失效, 视为成功
	}
	snapshot := item.Value()
	s.cache.Delete(strings.TrimSpace(token))
	s.clearCurrentUser(snapshot.UserID)
	s.info("session: logout", logkit.F("id", snapshot.UserID), logkit.F("username", snapshot.Username))
	s.emit(event.TopicSessionLogout, event.SessionLogout{
		UserID: snapshot.UserID, Username: snapshot.Username, At: s.now().UnixMilli(),
	})
	return nil
}

// Resume 启动恢复: 用 login_logs 里该 token 的最后一次成功登录时间判断是否仍有效.
func (s *Service) Resume(_ domaincontext.Context, token string) (Session, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, apperr.New(apperr.CodeNotFound)
	}
	if !s.config().resumeOnStart {
		return Session{}, invalid("session resume disabled by settings")
	}

	rows, err := s.queryLogins(`SELECT id, user_id, username, result, reason, token_hash, created_at FROM login_logs
        WHERE token_hash = ? AND result = 'success' ORDER BY created_at DESC LIMIT 1`, authkit.HashToken(token))
	if err != nil {
		return Session{}, err
	}
	if len(rows) == 0 {
		return Session{}, invalid("session not found, please sign in again")
	}
	loggedAt := time.UnixMilli(rows[0].CreatedAt)
	remaining := loggedAt.Add(s.config().expiration).Sub(s.now())
	if remaining <= 0 {
		s.writeLoginLog(rows[0].UserID, rows[0].Username, "failed", "expired", "")
		return Session{}, invalid("session expired, please sign in again")
	}

	row, err := s.findUserByID(rows[0].UserID)
	if err != nil {
		return Session{}, err
	}
	ttl := s.capTTL(loggedAt, remaining)
	if ttl <= 0 {
		return Session{}, invalid("session expired, please sign in again")
	}
	expiresAt := s.now().Add(ttl)
	s.cache.Set(token, entry{
		UserID: row.ID, Username: row.Username, CreatedAt: loggedAt, ExpiresAt: expiresAt,
	}, ttl)
	s.setCurrentUser(row.ID)
	return Session{Token: token, User: row.public(), ExpiresAt: expiresAt.UnixMilli(),
		LoginAt: loggedAt.UnixMilli()}, nil
}

// Current 校验会话并按配置滑动续期, 返回当前用户.
func (s *Service) Current(_ domaincontext.Context, token string) (serviceuser.User, error) {
	token = strings.TrimSpace(token)
	item := s.cache.Get(token)
	if item == nil {
		return serviceuser.User{}, invalid("session not found, please sign in again")
	}
	current := item.Value()

	if s.config().slidingExpire {
		ttl := s.capTTL(current.CreatedAt, s.config().expiration)
		if ttl <= 0 {
			s.cache.Delete(token)
			return serviceuser.User{}, invalid("session expired, please sign in again")
		}
		next := entry{UserID: current.UserID, Username: current.Username,
			CreatedAt: current.CreatedAt, ExpiresAt: s.now().Add(ttl)}
		s.cache.Set(token, next, ttl)
	}

	row, err := s.findUserByID(current.UserID)
	if err != nil {
		return serviceuser.User{}, err
	}
	return row.public(), nil
}

// ChangePassword 修改当前会话用户的口令, 成功后该用户的所有会话失效.
func (s *Service) ChangePassword(_ domaincontext.Context, token, oldPassword, newPassword string) error {
	token = strings.TrimSpace(token)
	item := s.cache.Get(token)
	if item == nil {
		return invalid("session not found, please sign in again")
	}
	current := item.Value()
	row, err := s.findUserByID(current.UserID)
	if err != nil {
		return err
	}
	if !authkit.VerifyPassword(row.Password, oldPassword) {
		return invalid("bad password for %q", row.Username)
	}
	if len(newPassword) < authkit.MinPasswordLen {
		return invalid(`password must be at least %d chars`, authkit.MinPasswordLen)
	}
	hash, err := authkit.HashPassword(newPassword)
	if err != nil {
		s.fail("changePassword", row.Username, err)
		return err
	}
	if _, err := s.exec(`UPDATE users SET password = ?, updated_at = ? WHERE id = ? AND deleted_at = 0`,
		hash, s.now().UnixMilli(), row.ID); err != nil {
		s.fail("changePassword", row.Username, err)
		return err
	}
	s.info("session: password changed", logkit.F("id", row.ID), logkit.F("username", row.Username))

	// 旧会话一律失效(包括当前这条): 之后需用新口令重新登录
	for key, cached := range s.cache.Items() {
		if cached.Value().UserID == row.ID {
			s.cache.Delete(key)
		}
	}
	return nil
}

// ---- 内部: 会话缓存 ----

// openSession 建立新会话(创建令牌 + 写入缓存), 返回对外形态.
func (s *Service) openSession(row userRow) (Session, error) {
	token, err := authkit.NewToken()
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	ttl := s.capTTL(now, s.config().expiration)
	if ttl <= 0 {
		return Session{}, invalid("session expiration misconfigured")
	}
	expiresAt := now.Add(ttl)
	s.cache.Set(token, entry{UserID: row.ID, Username: row.Username, CreatedAt: now, ExpiresAt: expiresAt},
		ttl)
	return Session{Token: token, User: row.public(), ExpiresAt: expiresAt.UnixMilli(), LoginAt: now.UnixMilli()}, nil
}

// capTTL 把 TTL 收敛到绝对上限内(从会话创建时间起算 7 天).
func (s *Service) capTTL(createdAt time.Time, ttl time.Duration) time.Duration {
	remaining := absoluteCap - s.now().Sub(createdAt)
	if remaining <= 0 {
		return 0
	}
	if ttl > remaining {
		return remaining
	}
	return ttl
}

// onEvicted 是 ttlcache 的过期钩子: 只对"到期"发事件(主动删除/换出不算).
func (s *Service) onEvicted(_ stdctx.Context, reason ttlcache.EvictionReason, item *ttlcache.Item[string, entry]) {
	if reason != ttlcache.EvictionReasonExpired {
		return
	}
	snapshot := item.Value()
	s.info("session: expired", logkit.F("id", snapshot.UserID), logkit.F("username", snapshot.Username))
	s.emit(event.TopicSessionExpired, event.SessionExpired{
		UserID: snapshot.UserID, Username: snapshot.Username, At: s.now().UnixMilli(),
	})
}

// ---- 内部: 配置读取(直接经 settings engine, 不调用其它 srv) ----

// toInt 把设置值(JSON 数字)转成 int; 无法识别时返回 0.
func toInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

// toInt2 把 get 的返回值转成 int.
func toInt2(value any, ok bool) int {
	if !ok {
		return 0
	}
	return toInt(value)
}

// ---- 内部: 用户与登录日志的读写 ----

const userColumns = `SELECT id, username, password, nickname, created_at, updated_at FROM users`

func (s *Service) findUserByUsername(username string) (userRow, error) {
	rows := []userRow{}
	if _, err := s.query(&rows, userColumns+` WHERE username = ? AND deleted_at = 0`, username); err != nil {
		return userRow{}, err
	}
	if len(rows) == 0 {
		return userRow{}, notFound(fmt.Sprintf("user %q", username))
	}
	return rows[0], nil
}

func (s *Service) findUserByID(id string) (userRow, error) {
	rows := []userRow{}
	if _, err := s.query(&rows, userColumns+` WHERE id = ? AND deleted_at = 0`, id); err != nil {
		return userRow{}, err
	}
	if len(rows) == 0 {
		return userRow{}, notFound(fmt.Sprintf("user %q", id))
	}
	return rows[0], nil
}

func (s *Service) queryLogins(sqlText string, args ...any) ([]loginRow, error) {
	rows := []loginRow{}
	if _, err := s.query(&rows, sqlText, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

// writeLoginLog 记录一次登录尝试(成功/失败都记; 失败原因用于审计与排查).
func (s *Service) writeLoginLog(userID, username, result, reason, tokenHash string) {
	if _, err := s.exec(`INSERT INTO login_logs (id, user_id, username, result, reason, token_hash, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)`, s.newID(), userID, username, result, reason, tokenHash, s.now().UnixMilli()); err != nil {
		s.deliver(domainlog.LevelWarn, "session: login log failed",
			logkit.F("username", username), logkit.F("err", err))
	}
}

// ---- 内部: store 访问(读一律 CmdQuery, 见 v17 §16) ----

func (s *Service) exec(sqlText string, args ...any) (domainstore.ExecResult, error) {
	value, err := s.store.Do(stdctx.Background(), domainstore.Task{
		Cmd:  domainstore.CmdExec,
		Args: domainstore.ExecArgs{SQL: sqlText, Args: args},
	})
	if err != nil {
		return domainstore.ExecResult{}, err
	}
	result, ok := cast.As[domainstore.ExecResult](value)
	if !ok {
		return domainstore.ExecResult{}, fmt.Errorf("session: unexpected exec result %T", value)
	}
	return result, nil
}

func (s *Service) query(into any, sqlText string, args ...any) (domainstore.QueryResult, error) {
	value, err := s.store.Do(stdctx.Background(), domainstore.Task{
		Cmd:  domainstore.CmdQuery,
		Args: domainstore.QueryArgs{SQL: sqlText, Args: args, Into: into},
	})
	if err != nil {
		return domainstore.QueryResult{}, err
	}
	result, ok := cast.As[domainstore.QueryResult](value)
	if !ok {
		return domainstore.QueryResult{}, fmt.Errorf("session: unexpected query result %T", value)
	}
	return result, nil
}

// ---- 内部: 事件, 日志, 错误, id ----

func (s *Service) emit(topic event.Topic, payload any) {
	if s.ebus != nil {
		s.ebus.Emit(topic, payload)
	}
}

func (s *Service) info(msg string, fields ...domainlog.Field) {
	s.deliver(domainlog.LevelInfo, msg, fields...)
}

func (s *Service) fail(op, subject string, err error) {
	s.deliver(domainlog.LevelError, "session: failed", logkit.F("op", op), logkit.F("subject", subject), logkit.F("err", err))
}

func (s *Service) deliver(level domainlog.Level, msg string, fields ...domainlog.Field) {
	if s.logger == nil {
		return
	}
	s.logger.Deliver(logkit.New(level, msg, fields...))
}

func notFound(what string) error {
	return fmt.Errorf("%w: %s not found", apperr.New(apperr.CodeNotFound), what)
}

func isNotFound(err error) bool {
	var target *apperr.Error
	if ok := asAppError(err, &target); ok && target != nil {
		return target.Code == apperr.CodeNotFound
	}
	return false
}

func asAppError(err error, target **apperr.Error) bool {
	for err != nil {
		if typed, ok := err.(*apperr.Error); ok {
			*target = typed
			return true
		}
		unwrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapped.Unwrap()
	}
	return false
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", apperr.New(apperr.CodeServiceFail), fmt.Sprintf(format, args...))
}

func validUsername(username string) bool {
	for _, r := range username {
		valid := r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !valid {
			return false
		}
	}
	return true
}

// newID 生成时间有序的字符串 id(与 proxy/user 同法).
func newID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%013x%s", time.Now().UnixMilli(), hex.EncodeToString(buf))
}
