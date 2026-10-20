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
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// Key 是工作区服务在 srvs 中的键.
var Key = key.New[*Service]("service.workspace")

const (
	maxNameLen = 64
	// defaultName 是用户首次使用(或删空后)自动创建的工作区名。
	defaultName = "Default"
)

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
	store  domainstore.Store
	logger domainlog.Logger
	now    func() time.Time
	newID  func() string

	// currentUser 返回当前登录用户 id(会话服务提供); 为空时无法区分归属。
	currentUser func() string

	mu      sync.RWMutex
	current map[string]string // userID -> workspaceID(内存, 不落库)
}

// New 创建工作区服务.
func New(store domainstore.Store, logger domainlog.Logger) *Service {
	return &Service{
		store:   store,
		logger:  logger,
		now:     time.Now,
		newID:   newID,
		current: map[string]string{},
	}
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

// List 返回当前用户的工作区(按创建时间升序); 没有则自动创建一个 Default。
func (s *Service) List(_ domaincontext.Context) ([]Workspace, error) {
	uid, err := s.requireUser()
	if err != nil {
		return nil, err
	}
	items, err := s.list(uid)
	if err != nil {
		return nil, err
	}
	// 不自动创建工作区(由用户显式添加); 也不自动选中 —— 未选中时由调用方约束资源操作。
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
	return nil
}

// ---- 内部 ----

func (s *Service) requireUser() (string, error) {
	uid := s.userID()
	if uid == "" {
		return "", invalid("workspace requires a signed-in user")
	}
	return uid, nil
}

func (s *Service) list(userID string) ([]Workspace, error) {
	items := []Workspace{}
	if _, err := s.query(&items, `SELECT id, name, created_at, updated_at FROM workspaces
        WHERE user_id = ? AND deleted_at = 0 ORDER BY created_at ASC`, userID); err != nil {
		return nil, err
	}
	return items, nil
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

	now := s.now().UnixMilli()
	item := Workspace{ID: s.newID(), Name: normalized, CreatedAt: now, UpdatedAt: now}
	if _, err := s.exec(`INSERT INTO workspaces (id, name, user_id, created_at, updated_at, deleted_at)
        VALUES (?, ?, ?, ?, ?, 0)`, item.ID, item.Name, userID, now, now); err != nil {
		return Workspace{}, err
	}
	s.info("workspace: created", logkit.F("id", item.ID), logkit.F("name", item.Name), logkit.F("user", userID))
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
