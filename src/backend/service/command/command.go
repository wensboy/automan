// Package command 是命令系统: 前端 search 只负责把命令 id 交给 web, 由本服务路由到内部 handler.
//
// 约定(docs 见 data/conf/command.json):
//   - 命令清单只取 **runtime** 段(cli 段是程序 CLI 的解析配置, 与本链路无关);
//   - 命令 id 形如 "<group>.<cmd>"(如 setting.open) —— group 来自 runtime.groups, cmd 是该组下的 cmd 字段;
//   - 命令不带参数, 后端执行后返回 Result.Action, 由前端按 action 做 UI 变化(类似 VSCode 的命令);
//   - handler **不导出**: 只有 Run/Register 是外部入口; 后续扩展指令在代码层通过 Register 接入, 复用同一格式。
package command

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"time"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// Key 是命令服务在 srvs 中的键.
var Key = key.New[*Service]("service.command")

// Entry 是命令清单里的一条(字段与 command.json 对应).
type Entry struct {
	Cmd   string `json:"cmd"`
	Label string `json:"label"`
	KBD   string `json:"kbd"`
}

// Item 是发给前端的命令(带 group 与全局唯一 id).
type Item struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	Cmd   string `json:"cmd"`
	Label string `json:"label"`
	KBD   string `json:"kbd"`
}

// Group 是命令分组(顺序取自 runtime.groups).
type Group struct {
	Name  string `json:"name"`
	Items []Item `json:"items"`
}

// Result 是命令执行结果: Action 供前端做 UI 变化, Message 用于友好反馈(可空).
type Result struct {
	OK      bool   `json:"ok"`
	Action  string `json:"action"`
	Target  string `json:"target"`
	Message string `json:"message"`
}

// Request 是传给 handler 的上下文信息(当前命令均无参数, 预留 Target).
type Request struct {
	ID    string
	Group string
	Cmd   string
}

// Handler 是内部处理器: 默认不导出, 只有 Register 能把它挂到路由表.
type Handler func(domaincontext.Context, Request) (Result, error)

// Service 是命令服务: 清单来自嵌入的 data/conf/command.json, 路由表在构造时装配.
type Service struct {
	data   fs.FS
	logger domainlog.Logger
	now    func() time.Time

	mu       sync.RWMutex
	handlers map[string]Handler
	order    []string
}

const commandPath = "data/conf/command.json"

// New 创建命令服务并装配内置 handler.
func New(data fs.FS, logger domainlog.Logger) *Service {
	s := &Service{
		data:     data,
		logger:   logger,
		now:      time.Now,
		handlers: map[string]Handler{},
	}
	s.registerBuiltin()
	return s
}

// Register 是扩展点: 代码层面接入新命令(格式与内置一致: "<group>.<cmd>").
func (s *Service) Register(id string, handler Handler) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" || handler == nil {
		return invalid("command id and handler are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.handlers[trimmed]; exists {
		return invalid("command %q already registered", trimmed)
	}
	s.handlers[trimmed] = handler
	s.order = append(s.order, trimmed)
	return nil
}

// List 返回 runtime 段的命令分组(顺序与配置文件一致; cli 段忽略).
func (s *Service) List(_ domaincontext.Context) ([]Group, error) {
	catalog, err := s.catalog()
	if err != nil {
		s.fail("list", err)
		return nil, err
	}

	groups := make([]Group, 0, len(catalog.Groups))
	for _, name := range catalog.Groups {
		entries := catalog.Runtime[name]
		items := make([]Item, 0, len(entries))
		for _, entry := range entries {
			cmd := strings.TrimSpace(entry.Cmd)
			if cmd == "" {
				continue
			}
			items = append(items, Item{
				ID: name + "." + cmd, Group: name, Cmd: cmd, Label: entry.Label, KBD: entry.KBD,
			})
		}
		groups = append(groups, Group{Name: name, Items: items})
	}
	s.info("command: listed", logkit.F("groups", len(groups)))
	return groups, nil
}

// Run 执行一条命令: web 只把 id 传进来, 具体行为由路由表决定.
func (s *Service) Run(_ domaincontext.Context, id string) (Result, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return Result{}, invalid("command id is required")
	}
	group, cmd, ok := strings.Cut(trimmed, ".")
	if !ok || group == "" || cmd == "" {
		return Result{}, invalid(`command id must be "<group>.<cmd>", got %q`, trimmed)
	}

	s.mu.RLock()
	handler, exists := s.handlers[trimmed]
	s.mu.RUnlock()
	if !exists {
		s.fail("run", fmt.Errorf("unhandled command %q", trimmed))
		return Result{}, invalid("command %q is not implemented", trimmed)
	}

	result, err := handler(nil, Request{ID: trimmed, Group: group, Cmd: cmd})
	if err != nil {
		s.fail("run", err)
		return Result{}, err
	}
	if !result.OK {
		result.OK = true
	}
	s.info("command: executed", logkit.F("id", trimmed), logkit.F("action", result.Action))
	return result, nil
}

// ---- 内部: 清单解析与内置 handler ----

type runtimeSpec struct {
	Groups  []string           `json:"groups"`
	Runtime map[string][]Entry `json:"runtime"`
	Meta    map[string]any     `json:"meta"`
	CLI     map[string]any     `json:"cli"`
}

// catalog 读取并解析 command.json: runtime 的动态分组键用 map 承载.
func (s *Service) catalog() (runtimeSpec, error) {
	payload, err := fs.ReadFile(s.data, commandPath)
	if err != nil {
		return runtimeSpec{}, fmt.Errorf("%w: command: read config: %v", apperr.New(apperr.CodeServiceFail), err)
	}
	var raw struct {
		Runtime json.RawMessage `json:"runtime"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return runtimeSpec{}, fmt.Errorf("%w: command: parse config: %v", apperr.New(apperr.CodeServiceFail), err)
	}
	var runtime map[string]json.RawMessage
	if err := json.Unmarshal(raw.Runtime, &runtime); err != nil {
		return runtimeSpec{}, fmt.Errorf("%w: command: parse runtime: %v", apperr.New(apperr.CodeServiceFail), err)
	}

	spec := runtimeSpec{Runtime: map[string][]Entry{}}
	if groupsRaw, ok := runtime["groups"]; ok {
		if err := json.Unmarshal(groupsRaw, &spec.Groups); err != nil {
			return runtimeSpec{}, fmt.Errorf("%w: command: parse groups: %v", apperr.New(apperr.CodeServiceFail), err)
		}
	}
	for _, name := range spec.Groups {
		if entriesRaw, ok := runtime[name]; ok {
			var entries []Entry
			if err := json.Unmarshal(entriesRaw, &entries); err != nil {
				return runtimeSpec{}, fmt.Errorf("%w: command: parse group %s: %v", apperr.New(apperr.CodeServiceFail), name, err)
			}
			spec.Runtime[name] = entries
		}
	}
	return spec, nil
}

// registerBuiltin 装配内置命令(与 command.json 的 runtime 条目一一对应).
// 命令只做"前端可感知的动作": 返回 Action, 不在这里操作 UI.
func (s *Service) registerBuiltin() {
	s.handlers["setting.open"] = func(_ domaincontext.Context, req Request) (Result, error) {
		return Result{OK: true, Action: "openSettings", Target: "settings", Message: "Settings"}, nil
	}
	s.handlers["setting.reset"] = func(_ domaincontext.Context, req Request) (Result, error) {
		return Result{OK: true, Action: "resetSettings", Target: "settings", Message: "Reset"}, nil
	}
	s.handlers["session.logout"] = func(_ domaincontext.Context, req Request) (Result, error) {
		return Result{OK: true, Action: "logout", Target: "session", Message: "Logout"}, nil
	}
}

func (s *Service) info(msg string, fields ...domainlog.Field) {
	if s.logger != nil {
		s.logger.Deliver(domainlog.Record{Time: s.now(), Level: domainlog.LevelInfo, Msg: msg, Fields: fields})
	}
}

func (s *Service) fail(op string, err error) {
	if s.logger != nil {
		s.logger.Deliver(domainlog.Record{
			Time: s.now(), Level: domainlog.LevelError, Msg: "command: " + op + " failed",
			Fields: []domainlog.Field{logkit.F("err", err)},
		})
	}
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", apperr.New(apperr.CodeServiceFail), fmt.Sprintf(format, args...))
}
