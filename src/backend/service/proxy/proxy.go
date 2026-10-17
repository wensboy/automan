// Package proxy 编排代理数据的 CRUD: 校验 + 落库(经 db store) + 操作日志.
//
// 边界(见 docs/design/v17.md):
//   - 只做**数据链路**: 不触发任何执行动作(启动/停止/重启代理属执行链路, 见 v17 §11);
//   - application 与 proxy 解耦(属 UI 层附着关系, 不进本表);
//   - endpoint 属 whistle 启动参数, 统一放在 rules(标准 JSON 字符串)里, 由 UI 编辑。
package proxy

import (
	stdctx "context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// Key 是代理服务在 srvs 中的键.
var Key = key.New[*Service]("service.proxy")

// 状态取值(与迁移里的 CHECK 一致; 不含过程态).
const (
	StatusRunning   = "running"
	StatusStopped   = "stopped"
	StatusError     = "error"
	StatusException = "exception"
)

// 校验上限.
const (
	maxNameLen   = 64
	maxTargetLen = 255
	maxTextLen   = 64 * 1024
)

// Proxy 是代理行(对前端).
// db tag 与列名一一对应: sqlx 默认映射只做小写化, 认不出 created_at -> CreatedAt。
type Proxy struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Target      string `json:"target" db:"target"`
	Status      string `json:"status" db:"status"`
	Rules       string `json:"rules" db:"rules"`
	Description string `json:"description" db:"description"`
	CreatedAt   int64  `json:"createdAt" db:"created_at"`
	UpdatedAt   int64  `json:"updatedAt" db:"updated_at"`
}

// Input 是写入入参(创建/更新共用).
type Input struct {
	Name        string `json:"name"`
	Target      string `json:"target"`
	Status      string `json:"status"`
	Rules       string `json:"rules"`
	Description string `json:"description"`
}

// Query 是列表查询: 搜索(name/target) + 状态过滤 + 分页.
type Query struct {
	Search   string `json:"search"`
	Status   string `json:"status"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// Page 是列表结果: 当前页 + 总数(供前端算页数) + 各状态条数(当前筛选结果, 供页脚统计徽标).
type Page struct {
	Items  []Proxy        `json:"items"`
	Total  int            `json:"total"`
	Counts map[string]int `json:"counts"`
}

// Service 编排代理 CRUD; store 与 logger 在构造时注入.
type Service struct {
	store  domainstore.Store
	logger domainlog.Logger
	now    func() time.Time
	newID  func() string
}

// New 创建代理服务.
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
	countWhere, countArgs := buildCountFilter(query)
	counts, err := s.statusCounts(countWhere, countArgs)
	if err != nil {
		return Page{}, err
	}
	if total == 0 {
		return Page{Items: []Proxy{}, Total: 0, Counts: counts}, nil
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

	rows := []Proxy{}
	listArgs := append(append([]any{}, args...), pageSize, offset)
	sqlText := selectColumns + ` WHERE ` + where + ` ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	if _, err := s.query(&rows, sqlText, listArgs...); err != nil {
		return Page{}, err
	}
	return Page{Items: rows, Total: total, Counts: counts}, nil
}

// Get 读取单条(已删除视为不存在).
func (s *Service) Get(_ domaincontext.Context, id string) (Proxy, error) {
	return s.mustGet(id)
}

// Create 创建一条代理.
func (s *Service) Create(_ domaincontext.Context, input Input) (Proxy, error) {
	normalized, err := validate(input)
	if err != nil {
		s.reject("create", input.Name, err)
		return Proxy{}, err
	}
	if err := s.ensureNameFree(normalized.Name, ""); err != nil {
		s.reject("create", normalized.Name, err)
		return Proxy{}, err
	}

	now := s.now().UnixMilli()
	id := s.newID()
	_, err = s.exec(`INSERT INTO proxies
        (id, name, target, status, rules, description, created_at, updated_at, deleted_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		id, normalized.Name, normalized.Target, normalized.Status, normalized.Rules, normalized.Description, now, now)
	if err != nil {
		s.fail("create", id, err)
		return Proxy{}, err
	}

	s.info("proxy: created", logkit.F("id", id), logkit.F("name", normalized.Name), logkit.F("target", normalized.Target))
	return s.mustGet(id)
}

// Update 更新一条代理(只改允许的字段).
func (s *Service) Update(_ domaincontext.Context, id string, input Input) (Proxy, error) {
	current, err := s.mustGet(id)
	if err != nil {
		return Proxy{}, err
	}
	normalized, err := validate(input)
	if err != nil {
		s.reject("update", id, err)
		return Proxy{}, err
	}
	if normalized.Name != current.Name {
		if err := s.ensureNameFree(normalized.Name, id); err != nil {
			s.reject("update", id, err)
			return Proxy{}, err
		}
	}

	now := s.now().UnixMilli()
	_, err = s.exec(`UPDATE proxies SET name = ?, target = ?, status = ?, rules = ?, description = ?, updated_at = ?
        WHERE id = ? AND deleted_at = 0`,
		normalized.Name, normalized.Target, normalized.Status, normalized.Rules, normalized.Description, now, id)
	if err != nil {
		s.fail("update", id, err)
		return Proxy{}, err
	}

	s.info("proxy: updated", logkit.F("id", id), logkit.F("name", normalized.Name),
		logkit.F("fields", strings.Join(changedFields(current, normalized), ",")))
	return s.mustGet(id)
}

// SetStatus 批量改状态: 只作用于选中的、未删除的行, 已处于目标状态的行不动, 返回实际改动条数。
// 工具栏"启动/停止"用: 启动 = status(running)、停止 = status(stopped)。
// 当前只落库 —— 真实的启停(whistle/libretto)属执行链路, 见 docs/design/v17.md §11。
func (s *Service) SetStatus(_ domaincontext.Context, ids []string, status string) (int, error) {
	target := strings.TrimSpace(status)
	if !validStatus(target) {
		return 0, invalid(`status must be one of running|stopped|error|exception, got %q`, target)
	}
	cleaned := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}
	if len(cleaned) == 0 {
		return 0, invalid("no proxy selected")
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(cleaned)), ",")
	args := make([]any, 0, len(cleaned)+3)
	args = append(args, target, s.now().UnixMilli(), target)
	for _, id := range cleaned {
		args = append(args, id)
	}
	result, err := s.exec(`UPDATE proxies SET status = ?, updated_at = ?
        WHERE deleted_at = 0 AND status <> ? AND id IN (`+placeholders+`)`, args...)
	if err != nil {
		s.fail("setStatus", target, err)
		return 0, err
	}
	if result.RowsAffected == 0 {
		s.info("proxy: status unchanged", logkit.F("status", target), logkit.F("count", 0))
		return 0, nil
	}
	s.info("proxy: status changed", logkit.F("status", target), logkit.F("count", result.RowsAffected))
	return int(result.RowsAffected), nil
}

// Delete 逻辑删除(置 deleted_at), 之后同名可重建.
func (s *Service) Delete(_ domaincontext.Context, id string) error {
	current, err := s.mustGet(id)
	if err != nil {
		return err
	}
	now := s.now().UnixMilli()
	result, err := s.exec(`UPDATE proxies SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at = 0`, now, now, id)
	if err != nil {
		s.fail("delete", id, err)
		return err
	}
	if result.RowsAffected == 0 {
		return s.notFound(id)
	}
	s.info("proxy: deleted", logkit.F("id", id), logkit.F("name", current.Name))
	return nil
}

// ---- 内部: 查询与校验 ----

const selectColumns = `SELECT id, name, target, status, rules, description, created_at, updated_at FROM proxies`

// searchClause 返回 name/target 的模糊搜索条件(无关键词时返回空串)。
func searchClause(query Query) (string, []any) {
	keyword := strings.TrimSpace(query.Search)
	if keyword == "" {
		return "", nil
	}
	like := "%" + keyword + "%"
	return "(name LIKE ? OR target LIKE ?)", []any{like, like}
}

// buildFilter 组装列表/总数的 WHERE: 固定排除已删除; 可选状态过滤 + name/target 模糊搜索。
func buildFilter(query Query) (string, []any) {
	where := []string{"deleted_at = 0"}
	args := []any{}
	if status := strings.TrimSpace(query.Status); status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	if clause, clauseArgs := searchClause(query); clause != "" {
		where = append(where, clause)
		args = append(args, clauseArgs...)
	}
	return strings.Join(where, " AND "), args
}

// buildCountFilter 组装状态统计的 WHERE: 只保留搜索条件 —— 统计要回答"各状态各有多少",
// 若也被状态过滤收窄, 选中某一状态后其余计数恒为 0, 页脚就没有意义了。
func buildCountFilter(query Query) (string, []any) {
	where := []string{"deleted_at = 0"}
	args := []any{}
	if clause, clauseArgs := searchClause(query); clause != "" {
		where = append(where, clause)
		args = append(args, clauseArgs...)
	}
	return strings.Join(where, " AND "), args
}

// validate 归一化并校验入参(空 status 视为 stopped)。
func validate(input Input) (Input, error) {
	out := Input{
		Name:        strings.TrimSpace(input.Name),
		Target:      strings.TrimSpace(input.Target),
		Status:      strings.TrimSpace(input.Status),
		Rules:       input.Rules,
		Description: input.Description,
	}
	if out.Status == "" {
		out.Status = StatusStopped
	}
	if out.Name == "" || len(out.Name) > maxNameLen {
		return Input{}, invalid(`name must be 1..%d chars`, maxNameLen)
	}
	if out.Target == "" || len(out.Target) > maxTargetLen {
		return Input{}, invalid(`target must be 1..%d chars`, maxTargetLen)
	}
	if !validStatus(out.Status) {
		return Input{}, invalid(`status must be one of running|stopped|error|exception, got %q`, out.Status)
	}
	if len(out.Rules) > maxTextLen {
		return Input{}, invalid(`rules exceeds %d bytes`, maxTextLen)
	}
	if len(out.Description) > maxTextLen {
		return Input{}, invalid(`description exceeds %d bytes`, maxTextLen)
	}
	return out, nil
}

func validStatus(status string) bool {
	switch status {
	case StatusRunning, StatusStopped, StatusError, StatusException:
		return true
	default:
		return false
	}
}

// changedFields 列出与当前行相比发生变化的列名(供操作日志)。
func changedFields(current Proxy, next Input) []string {
	fields := make([]string, 0, 5)
	if current.Name != next.Name {
		fields = append(fields, "name")
	}
	if current.Target != next.Target {
		fields = append(fields, "target")
	}
	if current.Status != next.Status {
		fields = append(fields, "status")
	}
	if current.Rules != next.Rules {
		fields = append(fields, "rules")
	}
	if current.Description != next.Description {
		fields = append(fields, "description")
	}
	if len(fields) == 0 {
		fields = append(fields, "-")
	}
	return fields
}

// ensureNameFree 保证未删除行里名字唯一(重复 -> 明确错误); 数据库的部分唯一索引是最后一道防线。
func (s *Service) ensureNameFree(name, excludeID string) error {
	total, err := s.count("name = ? AND deleted_at = 0 AND id <> ?", []any{name, excludeID})
	if err != nil {
		return err
	}
	if total > 0 {
		return invalid(`duplicate name %q`, name)
	}
	return nil
}

func (s *Service) mustGet(id string) (Proxy, error) {
	if strings.TrimSpace(id) == "" {
		return Proxy{}, s.notFound(id)
	}
	// 取单行也走切片查询: Into 只支持切片(sqlx.Select), 空结果即未找到
	items := []Proxy{}
	if _, err := s.query(&items, selectColumns+` WHERE id = ? AND deleted_at = 0`, id); err != nil {
		return Proxy{}, err
	}
	if len(items) == 0 {
		return Proxy{}, s.notFound(id)
	}
	return items[0], nil
}

func (s *Service) count(where string, args []any) (int, error) {
	values := []int{}
	if _, err := s.query(&values, `SELECT COUNT(*) FROM proxies WHERE `+where, args...); err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, nil
	}
	return values[0], nil
}

// statusCounts 统计当前筛选结果里各状态的条数(供页脚徽标; 与列表同源, 不受分页影响)。
func (s *Service) statusCounts(where string, args []any) (map[string]int, error) {
	result, err := s.queryRows(`SELECT status, COUNT(*) AS total FROM proxies WHERE `+where+` GROUP BY status`, args...)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{StatusRunning: 0, StatusStopped: 0, StatusError: 0, StatusException: 0}
	for _, row := range result.Rows {
		status, _ := row["status"].(string)
		if status == "" {
			continue
		}
		counts[status] = toInt(row["total"])
	}
	return counts, nil
}

func toInt(value any) int {
	switch typed := value.(type) {
	case int64:
		return int(typed)
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}

// ---- 内部: 经 store 的 db 指令访问 sqlite ----

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
		return domainstore.ExecResult{}, fmt.Errorf("proxy: unexpected exec result %T", value)
	}
	return result, nil
}

func (s *Service) query(into any, sqlText string, args ...any) (domainstore.QueryResult, error) {
	return s.runQuery(domainstore.QueryArgs{SQL: sqlText, Args: args, Into: into})
}

// queryRows 取通用行集(用于聚合类查询)。
func (s *Service) queryRows(sqlText string, args ...any) (domainstore.QueryResult, error) {
	return s.runQuery(domainstore.QueryArgs{SQL: sqlText, Args: args})
}

func (s *Service) runQuery(args domainstore.QueryArgs) (domainstore.QueryResult, error) {
	// 必须走 CmdQuery: 引擎里只有 query 处理器做 sqlx.Select 填充 Into;
	// CmdGet 是"取单行通用行集"(忽略 Into), 用它会导致读结果恒为空。
	value, err := s.store.Do(stdctx.Background(), domainstore.Task{Cmd: domainstore.CmdQuery, Args: args})
	if err != nil {
		return domainstore.QueryResult{}, err
	}
	result, ok := cast.As[domainstore.QueryResult](value)
	if !ok {
		return domainstore.QueryResult{}, fmt.Errorf("proxy: unexpected query result %T", value)
	}
	return result, nil
}

// ---- 内部: 日志(见 v17 §7) ----

func (s *Service) info(msg string, fields ...domainlog.Field) {
	s.deliver(domainlog.LevelInfo, msg, fields...)
}

func (s *Service) reject(op, name string, err error) {
	s.deliver(domainlog.LevelWarn, "proxy: rejected", logkit.F("op", op), logkit.F("name", name), logkit.F("err", err))
}

func (s *Service) fail(op, id string, err error) {
	s.deliver(domainlog.LevelError, "proxy: failed", logkit.F("op", op), logkit.F("id", id), logkit.F("err", err))
}

func (s *Service) deliver(level domainlog.Level, msg string, fields ...domainlog.Field) {
	if s.logger == nil {
		return
	}
	s.logger.Deliver(logkit.New(level, msg, fields...))
}

// ---- 内部: 错误与 id ----

func (s *Service) notFound(id string) error {
	return fmt.Errorf("%w: proxy %q not found", apperr.New(apperr.CodeNotFound), id)
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", apperr.New(apperr.CodeServiceFail), fmt.Sprintf(format, args...))
}

// newID 生成时间有序的字符串 id: 13 位十六进制毫秒 + 16 位十六进制随机。
// (时间前缀保证按创建顺序大致有序, 无需引入 ULID 依赖。)
func newID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%013x%s", time.Now().UnixMilli(), hex.EncodeToString(buf))
}
