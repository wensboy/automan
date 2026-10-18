// 启动链路: 由本服务决定启动参数(以 sqlite 为唯一基准), 经 Starter(组合根用 bridge 引擎适配)
// 交给 node 子进程拉起 whistle, 最后把让步后的参数写回数据库。见 docs/design/v19.md。
package proxy

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// Starter 是执行链路的窄接口: 服务不直接依赖 ipc/引擎, 由组合根适配(便于测试)。
type Starter interface {
	// Start 发起启动并等待 node 回 proxy.started / proxy.failed。
	Start(ctx stdctx.Context, req StartRequest) (StartInfo, error)
	// Stop 发起停止并等待回执; timeout 为 node 侧的强杀阈值(来自 bridge.json)。
	Stop(ctx stdctx.Context, proxyID string, timeout time.Duration) error
}

// StartRequest 是传给 node 的启动参数(键名与 whistle 启动参数一一对应)。
type StartRequest struct {
	ProxyID string `json:"proxyId"`
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	UIPort  int    `json:"uiport"`
	Timeout int    `json:"timeout"`
	DNS     string `json:"dnsServer"`
	BaseDir string `json:"basedir"`
}

// StartInfo 是 node 回传的启动结果(以实际生效值为准)。
type StartInfo struct {
	ProxyID string `json:"proxyId"`
	PID     int    `json:"pid"`
	Port    int    `json:"port"`
	UIPort  int    `json:"uiport"`
	URL     string `json:"url"`
}

// StartItem 是单条启动成功的对外形态(前端据此同步 target/端口显示)。
type StartItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Target string `json:"target"`
	Port   int    `json:"port"`
	UIPort int    `json:"uiport"`
	PID    int    `json:"pid"`
}

// StartFailure 是单条启动失败的原因。
type StartFailure struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// StartResult 是批量启动结果。
type StartResult struct {
	Items  []StartItem    `json:"items"`
	Failed []StartFailure `json:"failed"`
}

// Option 配置启动链路相关依赖。
type Option func(*Service)

// WithStarter 注入执行链路适配器(未注入时 Start 会明确报错, 不会静默成功)。
func WithStarter(starter Starter) Option { return func(s *Service) { s.starter = starter } }

// WithRoot 注入应用数据根(绝对路径): bridge.json 里的相对 basedir 会解析到它下面。
func WithRoot(root string) Option { return func(s *Service) { s.root = root } }

// WithData 注入只读数据源(data/): 服务**自行**读取模板与 basedir,
// 组合根只负责把数据源递进来, 不再替服务做解析(v19 §12)。
func WithData(fsys fs.FS) Option { return func(s *Service) { s.data = fsys } }

// loadData 读取本服务需要的两个只读数据文件: rules 模板 与 代理 basedir。
// 失败不阻断启动: 模板缺失时前端退化为 {}; basedir 缺失时用缺省 "proxies"。
func (s *Service) loadData() {
	if s.data == nil {
		return
	}
	if payload, err := fs.ReadFile(s.data, "data/default/proxy.rules.json"); err == nil {
		s.rulesTemplate = string(payload)
	} else {
		s.deliver(domainlog.LevelWarn, "proxy: rules template unavailable", logkit.F("err", err))
	}
	if payload, err := fs.ReadFile(s.data, "data/conf/bridge.json"); err == nil {
		var conf struct {
			Node struct {
				Services struct {
					Proxy struct {
						Root          string `json:"root"`
						StopTimeoutMs int    `json:"stopTimeoutMs"`
					} `json:"proxy"`
				} `json:"services"`
			} `json:"node"`
		}
		if err := json.Unmarshal(payload, &conf); err == nil {
			if root := conf.Node.Services.Proxy.Root; root != "" {
				s.proxyRoot = s.resolveBase(root)
			}
			if ms := conf.Node.Services.Proxy.StopTimeoutMs; ms > 0 {
				s.stopTimeout = time.Duration(ms) * time.Millisecond
			}
		} else {
			s.deliver(domainlog.LevelWarn, "proxy: bridge config invalid", logkit.F("err", err))
		}
	}
}

// CaptureTarget 是 capture 页可嵌入的 whistle webui 目标(仅运行中且配了 uiport 的代理)。
type CaptureTarget struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	UIPort int    `json:"uiport"`
	URL    string `json:"url"`
}

// Running 返回可用于抓包页的代理实例: status=running 且 rules.uiport > 0。
// 只从数据库推导 —— 重启后状态可能过期, 但"是否真的在监听"由前端 iframe 打开时暴露,
// 这里不做额外探测(避免每次进页面都去连端口)。
func (s *Service) Running(_ domaincontext.Context) ([]CaptureTarget, error) {
	rows := []Proxy{}
	if _, err := s.query(&rows, selectColumns+` WHERE deleted_at = 0 AND status = ? ORDER BY name ASC`, StatusRunning); err != nil {
		return nil, err
	}

	targets := make([]CaptureTarget, 0, len(rows))
	for _, row := range rows {
		parsed, err := parseRules(row.Rules)
		if err != nil {
			continue // 规则坏掉的行不参与展示(启动时也会失败)
		}
		uiPort := toIntValue(parsed.UIPort, 0)
		if uiPort <= 0 {
			continue // 未配置 uiport 的实例没有 webui, 不加入列表
		}
		host := webHost(parsed.Host)
		targets = append(targets, CaptureTarget{
			ID: row.ID, Name: row.Name, Host: host, UIPort: uiPort,
			URL: fmt.Sprintf("http://%s:%d", host, uiPort),
		})
	}
	return targets, nil
}

// webHost 把监听地址换成本机可访问的地址(0.0.0.0 / :: 这类通配地址对浏览器无意义)。
func webHost(host string) string {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" || trimmed == "0.0.0.0" || trimmed == "::" || trimmed == "[::]" {
		return "127.0.0.1"
	}
	return trimmed
}

// EnsureBaseDir 在**应用启动时**准备好代理 basedir(默认 <数据根>/proxies)。
// 放在启动期做, 而不是等第一次启动代理时临时创建:
//
//	· 该目录同时是 node 侧 spawn 的 cwd —— 缺失会让 spawn 报 ENOENT(伪装成"node 找不到");
//	· whistle 的 storage(-S)就落在它下面。
//
// 幂等: 已存在则无操作。
func (s *Service) EnsureBaseDir() error {
	target := s.proxyRoot
	if target == "" {
		target = s.resolveBase("proxies")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		s.fail("ensureBaseDir", target, err)
		return err
	}
	s.info("proxy: basedir ready", logkit.F("path", target))
	return nil
}

// resolveBase 把 bridge.json 里的 basedir 解成**绝对路径**:
// 绝对路径原样用; 相对路径落到数据根下(无数据根时退化为基于 cwd 的绝对路径)。
// 关键: 这个值会作为 node 侧 spawn 的 cwd —— 相对路径会让 cwd 指向不存在的位置, 直接 ENOENT。
func (s *Service) resolveBase(rel string) string {
	trimmed := strings.TrimSpace(rel)
	if trimmed == "" {
		trimmed = "proxies"
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	if s.root != "" {
		return filepath.Join(s.root, trimmed)
	}
	if abs, err := filepath.Abs(trimmed); err == nil {
		return abs
	}
	return trimmed
}

// Template 返回 rules 模板(供前端编辑器初始化)。
func (s *Service) Template(_ domaincontext.Context) string { return s.rulesTemplate }

// rules 是 proxies.rules 的解析形态(键名与 whistle 启动参数一致)。
type rules struct {
	Host      string `json:"host"`
	Port      any    `json:"port"`
	UIPort    any    `json:"uiport"`
	Storage   string `json:"storage"`
	Timeout   any    `json:"timeout"`
	DNSServer string `json:"dnsServer"`
}

// Start 批量启动: 解析参数 -> 让步 -> 交给 node -> 写回实际生效值(含 target = host:port)。
func (s *Service) Start(_ domaincontext.Context, ids []string) (StartResult, error) {
	if s.starter == nil {
		return StartResult{}, invalid("proxy starter not configured")
	}
	result := StartResult{Items: []StartItem{}, Failed: []StartFailure{}}
	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), s.startTimeout())
	defer cancel()

	for _, id := range ids {
		row, err := s.mustGet(id)
		if err != nil {
			result.Failed = append(result.Failed, StartFailure{ID: id, Reason: err.Error()})
			continue
		}
		req, err := s.prepareStartRequest(row)
		if err != nil {
			s.fail("start", row.Name, err)
			result.Failed = append(result.Failed, StartFailure{ID: row.ID, Name: row.Name, Reason: err.Error()})
			continue
		}

		info, err := s.starter.Start(ctx, req)
		if err != nil {
			s.markStatus(row.ID, StatusError)
			s.fail("start", row.Name, err)
			result.Failed = append(result.Failed, StartFailure{ID: row.ID, Name: row.Name, Reason: err.Error()})
			continue
		}

		// 以 node 回传的实际值为准(它可能再次让步)
		req.Port, req.UIPort = info.Port, info.UIPort
		target := targetFrom(req.Host, req.Port)
		if err := s.writeBack(row, req, target, StatusRunning); err != nil {
			s.fail("start.writeBack", row.Name, err)
			result.Failed = append(result.Failed, StartFailure{ID: row.ID, Name: row.Name, Reason: err.Error()})
			continue
		}
		s.index.claim(row.ID, req.BaseDir, req.Port, req.UIPort)
		s.info("proxy: started", logkit.F("id", row.ID), logkit.F("name", row.Name),
			logkit.F("port", req.Port), logkit.F("uiport", req.UIPort), logkit.F("dir", req.BaseDir))
		result.Items = append(result.Items, StartItem{
			ID: row.ID, Name: row.Name, Target: target, Port: req.Port, UIPort: req.UIPort, PID: info.PID,
		})
	}
	return result, nil
}

// Stop 批量停止: 只改状态, 进程收尾由 node runtime 负责(超时强杀, 见 bridge.json)。
func (s *Service) Stop(_ domaincontext.Context, ids []string) error {
	if s.starter == nil {
		return invalid("proxy starter not configured")
	}
	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), s.startTimeout())
	defer cancel()

	var firstErr error
	for _, id := range ids {
		row, err := s.mustGet(id)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := s.starter.Stop(ctx, row.ID, s.stopTimeout); err != nil {
			s.fail("stop", row.Name, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		s.markStatus(row.ID, StatusStopped)
		s.index.release(row.ID)
		s.info("proxy: stopped", logkit.F("id", row.ID), logkit.F("name", row.Name))
	}
	return firstErr
}

// prepareStartRequest 解析 rules、与模板合并, 并对 storage/port/uiport 做让步。
func (s *Service) prepareStartRequest(row Proxy) (StartRequest, error) {
	parsed, err := parseRules(row.Rules)
	if err != nil {
		return StartRequest{}, err
	}
	// 模板兜底(新行缺键时用模板值)
	if parsed.Host == "" {
		parsed.Host = s.templateRules().Host
	}
	req := StartRequest{
		ProxyID: row.ID,
		Name:    row.Name,
		Host:    parsed.Host,
		Port:    toIntValue(parsed.Port, toIntValue(s.templateRules().Port, 8899)),
		UIPort:  toIntValue(parsed.UIPort, toIntValue(s.templateRules().UIPort, 8899)),
		Timeout: toIntValue(parsed.Timeout, toIntValue(s.templateRules().Timeout, 30000)),
		DNS:     parsed.DNSServer,
		BaseDir: s.baseDirOf(row.Name),
	}
	if req.Host == "" {
		req.Host = "127.0.0.1"
	}

	// port / uiport: 冲突则线性 +100, 再随机(§9 C)
	req.Port = s.pickPort(req.Host, req.Port, row.ID)
	req.UIPort = s.pickPort(req.Host, req.UIPort, row.ID)
	for req.UIPort == req.Port {
		req.UIPort = s.pickPort(req.Host, req.UIPort+1, row.ID)
	}
	return req, nil
}

// writeBack 把让步后的参数写回规则与 target(target 恒为 host:port), 并更新状态。
// 失败重试 3 次(本地 sqlite, 同步执行足够快; 批量时的异步化见 v19 §4 的说明)。
func (s *Service) writeBack(row Proxy, req StartRequest, target, status string) error {
	payload, err := json.Marshal(map[string]any{
		"host": req.Host, "port": req.Port, "uiport": req.UIPort,
		"timeout": req.Timeout, "dnsServer": req.DNS,
	})
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(50*attempt) * time.Millisecond)
		}
		_, lastErr = s.exec(`UPDATE proxies SET rules = ?, target = ?, status = ?, updated_at = ?
            WHERE id = ? AND deleted_at = 0`, string(payload), target, status, s.now().UnixMilli(), row.ID)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (s *Service) markStatus(id, status string) {
	if _, err := s.exec(`UPDATE proxies SET status = ?, updated_at = ? WHERE id = ? AND deleted_at = 0`,
		status, s.now().UnixMilli(), id); err != nil {
		s.fail("status", id, err)
	}
}

func (s *Service) templateRules() rules {
	parsed, err := parseRules(s.rulesTemplate)
	if err != nil {
		return rules{}
	}
	return parsed
}

func (s *Service) startTimeout() time.Duration {
	if s.startWait <= 0 {
		return 15 * time.Second
	}
	return s.startWait
}

func parseRules(raw string) (rules, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return rules{}, nil
	}
	var parsed rules
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return rules{}, invalid("rules is not valid JSON: %v", err)
	}
	return parsed, nil
}

// toIntValue 兼容 JSON 里的数字与字符串两种写法(port/uiport 在模板里是字符串)。
func toIntValue(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

// ---- 内部: 内存映射与端口/目录让步(F) ----

type runtimeIndex struct {
	mu      sync.Mutex
	byID    map[string]string // proxy id -> storage
	storage map[string]string // storage -> proxy id
	ports   map[int]string    // port/uiport -> proxy id
}

func newRuntimeIndex() *runtimeIndex {
	return &runtimeIndex{byID: map[string]string{}, storage: map[string]string{}, ports: map[int]string{}}
}

func (i *runtimeIndex) claim(id, storage string, ports ...int) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if previous, ok := i.byID[id]; ok {
		delete(i.storage, previous)
	}
	i.byID[id] = storage
	if storage != "" {
		i.storage[storage] = id
	}
	for _, port := range ports {
		if port > 0 {
			i.ports[port] = id
		}
	}
}

func (i *runtimeIndex) release(id string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if storage, ok := i.byID[id]; ok {
		delete(i.storage, storage)
		delete(i.byID, id)
	}
	for port, owner := range i.ports {
		if owner == id {
			delete(i.ports, port)
		}
	}
}

func (i *runtimeIndex) ownerOfStorage(storage string) string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.storage[storage]
}

func (i *runtimeIndex) ownerOfPort(port int) string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.ports[port]
}

// migrateBaseDir 在**改名**时把旧的 <proxies>/<old> 迁到 <proxies>/<new>。
// 语义: DB 是唯一事实, 目录只是运行时数据 —— 迁移失败只告警, 不阻断更新。
// 运行中的代理不迁移(whistle 正在用那个目录), 需先停止。
func (s *Service) migrateBaseDir(oldName, newName string, running bool) {
	oldDir := s.baseDirOf(oldName)
	newDir := s.baseDirOf(newName)
	if oldDir == newDir {
		return
	}
	if _, err := os.Stat(oldDir); err != nil {
		return // 没有历史数据, 无需迁移
	}
	if running {
		s.deliver(domainlog.LevelWarn, "proxy: basedir migration skipped (running)",
			logkit.F("from", oldDir), logkit.F("to", newDir))
		return
	}
	if _, err := os.Stat(newDir); err == nil {
		s.deliver(domainlog.LevelWarn, "proxy: basedir migration skipped (target exists)",
			logkit.F("from", oldDir), logkit.F("to", newDir))
		return
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		s.deliver(domainlog.LevelWarn, "proxy: basedir migration failed",
			logkit.F("from", oldDir), logkit.F("to", newDir), logkit.F("err", err))
		return
	}
	s.info("proxy: basedir migrated", logkit.F("from", oldDir), logkit.F("to", newDir))
}

// baseDirOf 是每条代理的 basedir: <proxies>/<name>。
// whistle 会在其下再建 `.whistle/`(这一层去不掉, 见 v19 §23), 但已从
// `.whistle/custom_dirs/<编码后的绝对路径>` 收敛为 `<name>/.whistle/...`。
// name 在库内唯一, 因此无需再生成随机后缀。
func (s *Service) baseDirOf(name string) string {
	safe := strings.TrimSpace(name)
	safe = strings.ReplaceAll(safe, "/", "-")
	safe = strings.ReplaceAll(safe, string(filepath.Separator), "-")
	if safe == "" {
		safe = "proxy"
	}
	return filepath.Join(s.proxyRoot, safe)
}

// suggestStorage 生成基于代理名的 storage(尾部随机后缀, 避免目录冲突)。

// pickPort: 先线性试探到 +100(与真实 bind 结合), 仍失败则随机取一个可用端口。
func (s *Service) pickPort(host string, port int, owner string) int {
	if port <= 0 {
		port = 8899
	}
	for offset := 0; offset <= 100; offset++ {
		candidate := port + offset
		if candidate > 65535 {
			break
		}
		if other := s.index.ownerOfPort(candidate); other != "" && other != owner {
			continue
		}
		if portAvailable(host, candidate) {
			return candidate
		}
	}
	for attempt := 0; attempt < 100; attempt++ {
		candidate := 10000 + rand.Intn(50000)
		if other := s.index.ownerOfPort(candidate); other != "" && other != owner {
			continue
		}
		if portAvailable(host, candidate) {
			return candidate
		}
	}
	return port
}

// portAvailable 用"试绑"判断端口是否可用(真实可用性, 比只看数据库可靠)。
func portAvailable(host string, port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func randomSuffix() string {
	const alphabet = "0123456789abcdef"
	buf := make([]byte, 6)
	for index := range buf {
		buf[index] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(buf)
}
