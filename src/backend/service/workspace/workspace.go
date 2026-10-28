// Package workspace 编排工作区: user 1:n workspace 1:n proxy 的中间层(见 docs/design/v21.md).
//
// 单一职责: 工作区数据归本服务; **"当前工作区"的选择也记在本服务内**(内存 map[userID]workspaceID),
// 不落到其它服务, 也不要求调用方串联多个服务。
package workspace

import (
	stdctx "context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainnotice "github.com/wensboy/automan/backend/domain/notice"
	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

const (
	maxNameLen = 64
	// defaultLimit 是未配置 general.workspace.limit 时的兜底上限。
	defaultLimit = 32
)

// Key 是工作区服务在 srvs 中的键.
var Key = key.New[*Service]("service.workspace")

// Workspace 是一个工作区(对前端).
// 注意: 必须带 db tag —— store 引擎用 sqlx 按列名映射, 缺 tag 会报
// "missing destination name created_at in *[]workspace.Workspace"。
type Workspace struct {
	ID        string `json:"id" db:"id"`
	Name      string `json:"name" db:"name"`
	CreatedAt int64  `json:"createdAt" db:"created_at"`
	UpdatedAt int64  `json:"updatedAt" db:"updated_at"`
}

// Service 编排工作区数据与"当前工作区"。
type Service struct {
	notifier domainnotice.Publisher
	store    domainstore.Store
	settings domainsettings.Store
	logger   domainlog.Logger
	now      func() time.Time
	newID    func() string

	// currentUser 返回当前登录用户 id(会话服务提供); 为空时无法区分归属。
	currentUser func() string

	mu      sync.RWMutex
	current map[string]string // userID -> workspaceID(内存, 不落库)
}

// New 创建工作区服务.
// New 创建工作区服务: 只做基础初始化; 外部依赖经 WithXxx 链式注入。
func New() *Service {
	return &Service{now: time.Now, newID: newID, current: map[string]string{}}
}

// WithStore 注入数据库 store.
func (s *Service) WithStore(store domainstore.Store) *Service { s.store = store; return s }

// WithLogger 注入日志出口.
func (s *Service) WithLogger(logger domainlog.Logger) *Service { s.logger = logger; return s }

// WithSettings 注入设置引擎: 工作区数量上限来自 general.workspace.limit(可在 settings UI 调整)。
func (s *Service) WithSettings(settings domainsettings.Store) *Service {
	s.settings = settings
	return s
}

// keyLimit 是工作区数量上限的配置键(默认层/schema/前端元数据三处同键)。
const keyLimit = "general.workspace.limit"

// limit 返回当前生效的数量上限(未配置/非法时用 32)。
func (s *Service) limit() int {
	if s.settings != nil {
		if value, ok := s.settings.Get(keyLimit); ok {
			switch typed := value.(type) {
			case int:
				if typed > 0 {
					return typed
				}
			case int64:
				if typed > 0 {
					return int(typed)
				}
			case float64:
				if typed >= 1 {
					return int(typed)
				}
			}
		}
	}
	return defaultLimit
}

// WithCurrentUser 注入当前用户 id 的读取函数(未注入时所有操作返回明确错误).
func (s *Service) WithCurrentUser(fn func() string) *Service {
	s.currentUser = fn
	return s
}

func (s *Service) userID() string {
	if s.currentUser == nil {
		return ""
	}
	return strings.TrimSpace(s.currentUser())
}

// List 返回当前用户的工作区(按创建时间升序); 不自动创建、不自动选中。
func (s *Service) List(_ domaincontext.Context) ([]Workspace, error) {
	uid, err := s.requireUser()
	if err != nil {
		return nil, err
	}
	items := []Workspace{}
	if _, err := s.query(&items, `SELECT id, name, created_at, updated_at FROM workspaces
        WHERE user_id = ? AND deleted_at = 0 ORDER BY created_at ASC`, uid); err != nil {
		return nil, err
	}
	return items, nil
}

// Create 新建工作区(同用户内不允许重名).
func (s *Service) Create(_ domaincontext.Context, name string) (Workspace, error) {
	uid, err := s.requireUser()
	if err != nil {
		return Workspace{}, err
	}
	return s.create(uid, name)
}

// Delete 逻辑删除工作区; 若删的是当前工作区则清空当前(下次访问会 EnsureDefault)。
func (s *Service) Delete(_ domaincontext.Context, id string) error {
	uid, err := s.requireUser()
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return invalid("workspace id is required")
	}

	current, err := s.find(uid, trimmed)
	if err != nil {
		return err
	}
	now := s.now().UnixMilli()
	result, err := s.exec(`UPDATE workspaces SET deleted_at = ?, updated_at = ?
        WHERE id = ? AND user_id = ? AND deleted_at = 0`, now, now, trimmed, uid)
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return notFound(trimmed)
	}

	s.mu.Lock()
	if s.current[uid] == trimmed {
		delete(s.current, uid)
	}
	s.mu.Unlock()

	s.info("workspace: deleted", logkit.F("id", trimmed), logkit.F("name", current.Name))
	s.notify(domainnotice.LevelSuccess, "workspace", "Workspace deleted", current.Name)
	return nil
}

// Current 返回当前工作区 id(未选择时返回空串).
func (s *Service) Current(_ domaincontext.Context) string {
	uid := s.userID()
	if uid == "" {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current[uid]
}

// SetCurrent 切换当前工作区(校验归属)。
func (s *Service) SetCurrent(_ domaincontext.Context, id string) error {
	uid, err := s.requireUser()
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return invalid("workspace id is required")
	}
	item, err := s.find(uid, trimmed)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.current[uid] = item.ID
	s.mu.Unlock()
	s.info("workspace: switched", logkit.F("id", item.ID), logkit.F("name", item.Name))
	s.notify(domainnotice.LevelSuccess, "workspace", "Workspace switched", item.Name)
	return nil
}

func (s *Service) requireUser() (string, error) {
	uid := s.userID()
	if uid == "" {
		return "", invalid("workspace requires a signed-in user")
	}
	return uid, nil
}

func (s *Service) find(userID, id string) (Workspace, error) {
	items := []Workspace{}
	if _, err := s.query(&items, `SELECT id, name, created_at, updated_at FROM workspaces
        WHERE id = ? AND user_id = ? AND deleted_at = 0`, id, userID); err != nil {
		return Workspace{}, err
	}
	if len(items) == 0 {
		return Workspace{}, notFound(id)
	}
	return items[0], nil
}

func (s *Service) create(userID, name string) (Workspace, error) {
	normalized := strings.TrimSpace(name)
	switch {
	case normalized == "":
		return Workspace{}, invalid("workspace name is required")
	case len(normalized) > maxNameLen:
		return Workspace{}, invalid(`workspace name exceeds %d chars`, maxNameLen)
	}

	total, err := s.count(`SELECT COUNT(*) FROM workspaces WHERE user_id = ? AND name = ? AND deleted_at = 0`, userID, normalized)
	if err != nil {
		return Workspace{}, err
	}
	if total > 0 {
		return Workspace{}, invalid(`duplicate workspace name %q`, normalized)
	}

	// 数量上限: 只统计未删除的工作区
	existing, err := s.count(`SELECT COUNT(*) FROM workspaces WHERE user_id = ? AND deleted_at = 0`, userID)
	if err != nil {
		return Workspace{}, err
	}
	if limit := s.limit(); existing >= limit {
		s.fail("create", userID, fmt.Errorf("limit %d reached", limit))
		return Workspace{}, invalid("workspace limit reached (max %d)", limit)
	}

	now := s.now().UnixMilli()
	item := Workspace{ID: s.newID(), Name: normalized, CreatedAt: now, UpdatedAt: now}
	if _, err := s.exec(`INSERT INTO workspaces (id, name, user_id, created_at, updated_at, deleted_at)
        VALUES (?, ?, ?, ?, ?, 0)`, item.ID, item.Name, userID, now, now); err != nil {
		return Workspace{}, err
	}
	s.info("workspace: created", logkit.F("id", item.ID), logkit.F("name", item.Name), logkit.F("user", userID))
	s.notify(domainnotice.LevelSuccess, "workspace", "Workspace created", item.Name)
	return item, nil
}

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
		return domainstore.ExecResult{}, fmt.Errorf("workspace: unexpected exec result %T", value)
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
		return domainstore.QueryResult{}, fmt.Errorf("workspace: unexpected query result %T", value)
	}
	return result, nil
}

func (s *Service) count(sqlText string, args ...any) (int, error) {
	values := []int{}
	if _, err := s.query(&values, sqlText, args...); err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, nil
	}
	return values[0], nil
}

func (s *Service) info(msg string, fields ...domainlog.Field) {
	if s.logger != nil {
		s.logger.Deliver(domainlog.Record{Time: s.now(), Level: domainlog.LevelInfo, Msg: msg, Fields: fields})
	}
}

func (s *Service) fail(op, id string, err error) {
	if s.logger != nil {
		s.logger.Deliver(domainlog.Record{
			Time: s.now(), Level: domainlog.LevelError, Msg: "workspace: " + op + " failed",
			Fields: []domainlog.Field{logkit.F("id", id), logkit.F("err", err)},
		})
	}
}

func newID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%013x%s", time.Now().UnixMilli(), hex.EncodeToString(buf))
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", apperr.New(apperr.CodeServiceFail), fmt.Sprintf(format, args...))
}

func notFound(id string) error {
	return fmt.Errorf("%w: workspace %q not found", apperr.New(apperr.CodeNotFound), id)
}

var _ = errors.Is

// WithNotice 注入业务提示出口(可为 nil)。
func (s *Service) WithNotice(publisher domainnotice.Publisher) *Service {
	s.notifier = publisher
	return s
}

// notify 投递一条业务提示(未注入时静默跳过)。
func (s *Service) notify(level domainnotice.Level, label, title, content string) {
	if s.notifier == nil {
		return
	}
	_, _ = s.notifier.Push(domainnotice.Entry{Level: string(level), Label: label, Title: title, Content: content})
}
