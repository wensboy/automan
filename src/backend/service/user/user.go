// Package user 编排用户数据的 CRUD: 校验 + 落库(经 db store) + 操作日志.
//
// 与 session 解耦(见 docs/design/v18.md §11 I): 本包只管用户本身, 不涉及登录、会话缓存与口令校验流程;
// 会话侧需要读用户时自己查库, 两个服务之间不互相调用(§4 的 [!notice])。
package user

import (
	stdctx "context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/authkit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// Key 是用户服务在 srvs 中的键.
var Key = key.New[*Service]("service.user")

// 校验上限.
const (
	minUsernameLen = 3
	maxUsernameLen = 32
	maxNicknameLen = 64
)

// User 是用户行(对前端; 不含口令).
type User struct {
	ID        string `json:"id" db:"id"`
	Username  string `json:"username" db:"username"`
	Nickname  string `json:"nickname" db:"nickname"`
	CreatedAt int64  `json:"createdAt" db:"created_at"`
	UpdatedAt int64  `json:"updatedAt" db:"updated_at"`
}

// Input 是写入入参: 创建时 password 必填; 更新时忽略它(改密走 ResetPassword).
type Input struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

// Query 是列表查询: 搜索(用户名/展示名) + 分页.
type Query struct {
	Search   string `json:"search"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// Page 是列表结果.
type Page struct {
	Items []User `json:"items"`
	Total int    `json:"total"`
}

// Service 编排用户 CRUD; store 与 logger 在构造时注入.
type Service struct {
	store  domainstore.Store
	logger domainlog.Logger
	now    func() time.Time
	newID  func() string
}

// New 创建用户服务.
func New(store domainstore.Store, logger domainlog.Logger) *Service {
	return &Service{store: store, logger: logger, now: time.Now, newID: newID}
}

// List 返回满足条件的当前页与总数.
func (s *Service) List(_ domaincontext.Context, query Query) (Page, error) {
	where, args := buildFilter(query)
	total, err := s.count(where, args)
	if err != nil {
		return Page{}, err
	}
	if total == 0 {
		return Page{Items: []User{}, Total: 0}, nil
	}

	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := query.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize

	items := []User{}
	listArgs := append(append([]any{}, args...), pageSize, offset)
	if _, err := s.query(&items, selectColumns+` WHERE `+where+` ORDER BY updated_at DESC LIMIT ? OFFSET ?`, listArgs...); err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: total}, nil
}

// Get 读取单个用户(已删除视为不存在).
func (s *Service) Get(_ domaincontext.Context, id string) (User, error) {
	return s.mustGet(id)
}

// Create 创建用户(口令哈希后落库).
func (s *Service) Create(_ domaincontext.Context, input Input) (User, error) {
	normalized, err := validate(input, true)
	if err != nil {
		s.reject("create", input.Username, err)
		return User{}, err
	}
	if err := s.ensureUsernameFree(normalized.Username, ""); err != nil {
		s.reject("create", normalized.Username, err)
		return User{}, err
	}
	hash, err := authkit.HashPassword(input.Password)
	if err != nil {
		s.fail("create", input.Username, err)
		return User{}, err
	}

	now := s.now().UnixMilli()
	id := s.newID()
	if _, err := s.exec(`INSERT INTO users
        (id, username, password, nickname, created_at, updated_at, deleted_at)
        VALUES (?, ?, ?, ?, ?, ?, 0)`,
		id, normalized.Username, hash, normalized.Nickname, now, now); err != nil {
		s.fail("create", id, err)
		return User{}, err
	}
	s.info("user: created", logkit.F("id", id), logkit.F("username", normalized.Username))
	return s.mustGet(id)
}

// Update 更新用户资料(不含口令).
func (s *Service) Update(_ domaincontext.Context, id string, input Input) (User, error) {
	current, err := s.mustGet(id)
	if err != nil {
		return User{}, err
	}
	normalized, err := validate(input, false)
	if err != nil {
		s.reject("update", id, err)
		return User{}, err
	}
	if normalized.Username != current.Username {
		if err := s.ensureUsernameFree(normalized.Username, id); err != nil {
			s.reject("update", id, err)
			return User{}, err
		}
	}

	now := s.now().UnixMilli()
	if _, err := s.exec(`UPDATE users SET username = ?, nickname = ?, updated_at = ?
        WHERE id = ? AND deleted_at = 0`,
		normalized.Username, normalized.Nickname, now, id); err != nil {
		s.fail("update", id, err)
		return User{}, err
	}
	s.info("user: updated", logkit.F("id", id), logkit.F("username", normalized.Username))
	return s.mustGet(id)
}

// Delete 逻辑删除.
func (s *Service) Delete(_ domaincontext.Context, id string) error {
	current, err := s.mustGet(id)
	if err != nil {
		return err
	}
	now := s.now().UnixMilli()
	result, err := s.exec(`UPDATE users SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at = 0`, now, now, id)
	if err != nil {
		s.fail("delete", id, err)
		return err
	}
	if result.RowsAffected == 0 {
		return s.notFound(id)
	}
	s.info("user: deleted", logkit.F("id", id), logkit.F("username", current.Username))
	return nil
}

// ResetPassword 管理侧重置口令(不校验旧口令).
func (s *Service) ResetPassword(_ domaincontext.Context, id, newPassword string) error {
	current, err := s.mustGet(id)
	if err != nil {
		return err
	}
	if err := validatePassword(newPassword); err != nil {
		s.reject("resetPassword", id, err)
		return err
	}
	hash, err := authkit.HashPassword(newPassword)
	if err != nil {
		s.fail("resetPassword", id, err)
		return err
	}
	now := s.now().UnixMilli()
	if _, err := s.exec(`UPDATE users SET password = ?, updated_at = ? WHERE id = ? AND deleted_at = 0`, hash, now, id); err != nil {
		s.fail("resetPassword", id, err)
		return err
	}
	s.info("user: password reset", logkit.F("id", id), logkit.F("username", current.Username))
	return nil
}

// ---- 内部: 查询与校验 ----

const selectColumns = `SELECT id, username, nickname, created_at, updated_at FROM users`

func buildFilter(query Query) (string, []any) {
	where := []string{"deleted_at = 0"}
	args := []any{}
	if keyword := strings.TrimSpace(query.Search); keyword != "" {
		where = append(where, "(username LIKE ? OR nickname LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	return strings.Join(where, " AND "), args
}

// validate 归一化并校验入参; requirePassword 为 true 时校验口令(创建).
func validate(input Input, requirePassword bool) (Input, error) {
	out := Input{
		Username: strings.TrimSpace(input.Username),
		Password: input.Password,
		Nickname: strings.TrimSpace(input.Nickname),
	}
	if len(out.Username) < minUsernameLen || len(out.Username) > maxUsernameLen {
		return Input{}, invalid(`username must be %d..%d chars`, minUsernameLen, maxUsernameLen)
	}
	for _, r := range out.Username {
		valid := r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !valid {
			return Input{}, invalid(`username allows letters, digits, "_" and "-" only`)
		}
	}
	if len(out.Nickname) > maxNicknameLen {
		return Input{}, invalid(`nickname exceeds %d chars`, maxNicknameLen)
	}
	if requirePassword {
		if err := validatePassword(out.Password); err != nil {
			return Input{}, err
		}
	}
	return out, nil
}

func validatePassword(password string) error {
	if len(password) < authkit.MinPasswordLen {
		return invalid(`password must be at least %d chars`, authkit.MinPasswordLen)
	}
	if len(password) > authkit.MaxPasswordLen {
		return invalid(`password exceeds %d chars`, authkit.MaxPasswordLen)
	}
	return nil
}

// ensureUsernameFree 保证未删除行里用户名唯一; 数据库的部分唯一索引是最后一道防线.
func (s *Service) ensureUsernameFree(username, excludeID string) error {
	total, err := s.count("username = ? AND deleted_at = 0 AND id <> ?", []any{username, excludeID})
	if err != nil {
		return err
	}
	if total > 0 {
		return invalid(`duplicate username %q`, username)
	}
	return nil
}

func (s *Service) mustGet(id string) (User, error) {
	if strings.TrimSpace(id) == "" {
		return User{}, s.notFound(id)
	}
	items := []User{}
	if _, err := s.query(&items, selectColumns+` WHERE id = ? AND deleted_at = 0`, id); err != nil {
		return User{}, err
	}
	if len(items) == 0 {
		return User{}, s.notFound(id)
	}
	return items[0], nil
}

func (s *Service) count(where string, args []any) (int, error) {
	values := []int{}
	if _, err := s.query(&values, `SELECT COUNT(*) FROM users WHERE `+where, args...); err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, nil
	}
	return values[0], nil
}

// ---- 内部: 经 store 的 db 指令访问 sqlite(读一律 CmdQuery, 见 v17 §16) ----

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
		return domainstore.ExecResult{}, fmt.Errorf("user: unexpected exec result %T", value)
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
		return domainstore.QueryResult{}, fmt.Errorf("user: unexpected query result %T", value)
	}
	return result, nil
}

// ---- 内部: 日志与错误 ----

func (s *Service) info(msg string, fields ...domainlog.Field) {
	s.deliver(domainlog.LevelInfo, msg, fields...)
}

func (s *Service) reject(op, name string, err error) {
	s.deliver(domainlog.LevelWarn, "user: rejected", logkit.F("op", op), logkit.F("name", name), logkit.F("err", err))
}

func (s *Service) fail(op, id string, err error) {
	s.deliver(domainlog.LevelError, "user: failed", logkit.F("op", op), logkit.F("id", id), logkit.F("err", err))
}

func (s *Service) deliver(level domainlog.Level, msg string, fields ...domainlog.Field) {
	if s.logger == nil {
		return
	}
	s.logger.Deliver(logkit.New(level, msg, fields...))
}

func (s *Service) notFound(id string) error {
	return fmt.Errorf("%w: user %q not found", apperr.New(apperr.CodeNotFound), id)
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", apperr.New(apperr.CodeServiceFail), fmt.Sprintf(format, args...))
}

// newID 生成时间有序的字符串 id(与 proxy 同法): 13 位十六进制毫秒 + 16 位十六进制随机.
func newID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%013x%s", time.Now().UnixMilli(), hex.EncodeToString(buf))
}

var _ = errors.Is
