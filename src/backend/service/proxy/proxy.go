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
	"io/fs"
	"strings"
	"sync"
	"time"

	domainconfig "github.com/wensboy/automan/backend/domain/config"
	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainnotice "github.com/wensboy/automan/backend/domain/notice"
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
	// WorkspaceID 是归属工作区(1:n); 历史行为空串。
	WorkspaceID string `json:"workspaceId" db:"workspace_id"`
	CreatedAt   int64  `json:"createdAt" db:"created_at"`
	UpdatedAt   int64  `json:"updatedAt" db:"updated_at"`
}

// Input 是写入入参(创建/更新共用).
// Input 是创建/更新入参。**不含 status**: 状态由执行链路(Start/Stop)维护,
// 允许入参写状态会让"状态与事实不符"(见 docs/design/v23.md §22/§23)。
type Input struct {
	Name        string `json:"name"`
	Target      string `json:"target"`
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

// Service 编排代理 CRUD 与启动链路; store 与 logger 在构造时注入, 执行链路经 Option 注入.
type Service struct {
	store  domainstore.Store
	logger domainlog.Logger
	now    func() time.Time
	newID  func() string

	// 启动链路(v19)
	starter       Starter
	data          fs.FS
	rulesTemplate string
	// provider 是 conf 引擎(读 data/conf/*.json 的类型化视图); 服务不自己解析配置文件。
	provider domainconfig.Provider
	// root 是应用数据根(绝对路径): basedir/storage 都落在它下面。
	// 不落到这里的相对路径会被当成子进程的 cwd -> 目录不存在 -> spawn ENOENT。
	root      string
	proxyRoot string
	// currentUser 返回"当前登录用户 id"(会话服务提供); 为空表示无归属信息, 不做收束。
	currentUser func() string
	// currentWorkspace 返回"当前工作区 id"(工作区服务提供); 为空时不按工作区收束。
	currentWorkspace func() string
	stopTimeout      time.Duration
	startWait        time.Duration
	index            *runtimeIndex
	// notifier 是业务提示出口(可为 nil: 不产生提示)
	notifier domainnotice.Publisher
	// opMu 串行化启停: header 批量与行菜单可能**同时**发起(不同入口),
	// 并发会让同一实例被"启动中又被停止"或重复 claim, 极端情况下进程层面出错。
	// 见 docs/design/v23.md §26。
	opMu sync.Mutex
}

// New 创建代理服务.
// New 创建代理服务: 只做基础初始化; 外部依赖经 WithXxx 链式注入(顺序无关)。
func New() *Service {
	return &Service{
		now: time.Now, newID: newID,
		proxyRoot: "proxies", startWait: 15 * time.Second, index: newRuntimeIndex(),
	}
}

// WithStore 注入数据库 store.
func (s *Service) WithStore(store domainstore.Store) *Service { s.store = store; return s }

// WithLogger 注入日志出口.
func (s *Service) WithLogger(logger domainlog.Logger) *Service { s.logger = logger; return s }

// List 返回满足条件的当前页与总数.
func (s *Service) List(_ domaincontext.Context, query Query) (Page, error) {
	// 没有当前工作区: 不返回任何数据(避免跨工作区/跨用户误显示)
	if s.workspaceID() == "" {
		return Page{Items: []Proxy{}, Total: 0, Counts: map[string]int{}}, nil
	}
	where, args := s.scopeWhere(buildFilter(query))

	total, err := s.count(where, args)
	if err != nil {
		return Page{}, err
	}
	countWhere, countArgs := s.scopeWhere(buildCountFilter(query))
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
	if err := s.requireWorkspace(); err != nil {
		return Proxy{}, err
	}
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
	columns := "id, name, target, status, rules, description, created_at, updated_at, deleted_at"
	values := "?, ?, ?, ?, ?, ?, ?, ?, 0"
	args := []any{id, normalized.Name, normalized.Target, StatusStopped, normalized.Rules, normalized.Description, now, now}
	if uid := s.userID(); uid != "" {
		columns += ", user_id"
		values += ", ?"
		args = append(args, uid)
	}
	if wid := s.workspaceID(); wid != "" {
		columns += ", workspace_id"
		values += ", ?"
		args = append(args, wid)
	}
	_, err = s.exec(`INSERT INTO proxies (`+columns+`) VALUES (`+values+`)`, args...)
	if err != nil {
		s.fail("create", id, err)
		return Proxy{}, err
	}

	s.info("proxy: created", logkit.F("id", id), logkit.F("name", normalized.Name), logkit.F("target", normalized.Target))
	s.notify(domainnotice.LevelSuccess, "proxy", "Proxy created", normalized.Name)
	return s.mustGet(id)
}

// Update 更新一条代理(只改允许的字段).
func (s *Service) Update(_ domaincontext.Context, id string, input Input) (Proxy, error) {
	if err := s.requireWorkspace(); err != nil {
		return Proxy{}, err
	}
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
	where, whereArgs := s.userScope("id = ? AND deleted_at = 0")
	updateArgs := append([]any{normalized.Name, normalized.Target, normalized.Rules, normalized.Description, now, id}, whereArgs...)
	_, err = s.exec(`UPDATE proxies SET name = ?, target = ?, rules = ?, description = ?, updated_at = ?
        WHERE `+where, updateArgs...)
	if err != nil {
		s.fail("update", id, err)
		return Proxy{}, err
	}

	// 改名会换 basedir(<proxies>/<name>): 把旧目录迁过去, 避免历史数据留在旧名下
	if normalized.Name != current.Name {
		s.migrateBaseDir(current.Name, normalized.Name, current.Status == StatusRunning)
	}

	s.info("proxy: updated", logkit.F("id", id), logkit.F("name", normalized.Name),
		logkit.F("fields", strings.Join(changedFields(current, normalized), ",")))
	s.notify(domainnotice.LevelSuccess, "proxy", "Proxy updated", normalized.Name)
	return s.mustGet(id)
}

// Delete 逻辑删除(置 deleted_at), 之后同名可重建.
func (s *Service) Delete(_ domaincontext.Context, id string) error {
	if err := s.requireWorkspace(); err != nil {
		return err
	}
	current, err := s.mustGet(id)
	if err != nil {
		return err
	}
	now := s.now().UnixMilli()
	where, whereArgs := s.userScope("id = ? AND deleted_at = 0")
	deleteArgs := append([]any{now, now, id}, whereArgs...)
	result, err := s.exec(`UPDATE proxies SET deleted_at = ?, updated_at = ? WHERE `+where, deleteArgs...)
	if err != nil {
		s.fail("delete", id, err)
		return err
	}
	if result.RowsAffected == 0 {
		return s.notFound(id)
	}
	s.info("proxy: deleted", logkit.F("id", id), logkit.F("name", current.Name))
	s.notify(domainnotice.LevelSuccess, "proxy", "Proxy deleted", current.Name)
	return nil
}

// ---- 内部: 查询与校验 ----

const selectColumns = `SELECT id, name, target, status, rules, description, workspace_id, created_at, updated_at FROM proxies`

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

// validate 归一化并校验入参(status 不在入参内: 由执行链路维护)。
func validate(input Input) (Input, error) {
	out := Input{
		Name:        strings.TrimSpace(input.Name),
		Rules:       input.Rules,
		Description: input.Description,
	}
	if out.Name == "" || len(out.Name) > maxNameLen {
		return Input{}, invalid(`name must be 1..%d chars`, maxNameLen)
	}
	// target 不是用户输入: 始终由 rules 的 host + port 派生(库里的列只是冗余存储, 便于搜索/展示)
	out.Target = deriveTarget(out.Rules)
	if len(out.Target) > maxTargetLen {
		return Input{}, invalid(`target exceeds %d chars (check rules.host/port)`, maxTargetLen)
	}
	if len(out.Rules) > maxTextLen {
		return Input{}, invalid(`rules exceeds %d bytes`, maxTextLen)
	}
	if len(out.Description) > maxTextLen {
		return Input{}, invalid(`description exceeds %d bytes`, maxTextLen)
	}
	return out, nil
}

// deriveTarget 从 rules 的 host + port 派生 target(与启动链路写回时用的是同一个函数)。
// rules 缺失/非法时返回空串 —— 由上层决定是否允许(当前允许, 因为新建时可能还没填 rules)。
func deriveTarget(rulesJSON string) string {
	parsed, err := parseRules(rulesJSON)
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(parsed.Host)
	if host == "" {
		return ""
	}
	return targetFrom(host, toIntValue(parsed.Port, 0))
}

// targetFrom 是 target 的唯一拼装口径(host:port); port <= 0 时只给 host。
func targetFrom(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if port <= 0 {
		return host
	}
	return fmt.Sprintf("%s:%d", host, port)
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
	where, whereArgs := s.userScope("name = ? AND deleted_at = 0 AND id <> ?")
	if wid := s.workspaceID(); wid != "" {
		where = where + " AND workspace_id = ?"
		whereArgs = append(whereArgs, wid)
	}
	total, err := s.count(where, append([]any{name, excludeID}, whereArgs...))
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
	queryArgs := []any{id}
	clause := `deleted_at = 0`
	if uid := s.userID(); uid != "" {
		clause += ` AND user_id = ?`
		queryArgs = append(queryArgs, uid)
	}
	if _, err := s.query(&items, selectColumns+` WHERE id = ? AND `+clause, queryArgs...); err != nil {
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
