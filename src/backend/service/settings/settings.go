// Package settings 编排设置服务。
//
// 职责: 对外提供"读生效值 / 写运行态 / 落盘"三类能力, 并在写入前按 schema 的键表做类型校验。
// 依赖边界:
//   - **内部持有** settings 引擎(构造时注入, 不逐次解析上下文), 只经 domain/settings.Store 访问;
//   - 键集合与类型来自 **`data/schema/settings.user.schema.json`**(见 v15 第 5 节"路线 1"),
//     由 internal/kits/schemakit 解析;
//   - 默认值来自 **`data/defaults/settings.user.json`**(见 v15 第 12 节 A: 候选 B),
//     与用户文件模板分离 —— 前者是"默认值", 后者是"写到用户目录的初始文件"。
package settings

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/key"
	domainnotice "github.com/wensboy/automan/backend/domain/notice"
	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/schemakit"
	"github.com/wensboy/automan/backend/internal/kits/storekit"
)

// ViewDefault 表示取值来自默认值层(data/defaults), 位于引擎三视图之外.
const ViewDefault = "default"

// Key 是设置服务在 srvs 中的键.
var Key = key.New[*Service]("service.settings")

// Entry 是单个配置项对外的形态: 生效值 + 来源层.
type Entry struct {
	Value any    `json:"value"`
	View  string `json:"view"`
}

// Service 编排设置读写; 引擎与 schema 键表在构造时注入.
type Service struct {
	notifier domainnotice.Publisher
	store    domainsettings.Store
	files    domainstore.Store // 用户目录(动态选项的文件来源, 例 theme.json)
	data     fs.FS             // 内嵌资源(缺省 font.json 等)
	fields   map[string]schemakit.Field
	defaults map[string]any
}

// New 创建设置服务.
// fields 为 schema 解析出的键表(决定"哪些键可读写、各是什么类型"), defaults 为默认值(点号键)。
// New 创建设置服务: 只做基础初始化; 外部依赖经 WithXxx 链式注入。
func New() *Service {
	return &Service{defaults: map[string]any{}}
}

// WithStore 注入设置引擎.
func (s *Service) WithStore(store domainsettings.Store) *Service { s.store = store; return s }

// WithFiles 注入用户目录文件存储(供动态选项读取外部文件, 例 theme.json).
func (s *Service) WithFiles(files domainstore.Store) *Service {
	s.files = files
	return s
}

// WithFields 注入 schema 字段表.
func (s *Service) WithFields(fields map[string]schemakit.Field) *Service {
	s.fields = fields
	return s
}

// WithDefaults 注入默认值层(nil 视为空表).
func (s *Service) WithDefaults(defaults map[string]any) *Service {
	if defaults == nil {
		defaults = map[string]any{}
	}
	s.defaults = defaults
	return s
}

// List 返回全部可配置键的生效值: 引擎有值取引擎值(含来源层), 否则回落默认值; 两者都无则不出现在结果里。
func (s *Service) List(_ domaincontext.Context) (map[string]Entry, error) {
	result := make(map[string]Entry, len(s.fields))
	for name := range s.fields {
		if entry, ok := s.read(name); ok {
			result[name] = entry
		}
	}
	return result, nil
}

// Get 读取单个配置项.
func (s *Service) Get(_ domaincontext.Context, name string) (Entry, error) {
	if _, ok := s.fields[name]; !ok {
		return Entry{}, s.unknownKey(name)
	}
	entry, ok := s.read(name)
	if !ok {
		return Entry{}, apperr.New(apperr.CodeNotFound)
	}
	return entry, nil
}

// Set 只写运行态(runtime 层, 进程内生效, 不落盘); 键与类型必须符合 schema.
func (s *Service) Set(_ domaincontext.Context, name string, value any) error {
	if err := s.validate(name, value); err != nil {
		return err
	}
	return s.store.Set(name, value)
}

// Reset 抹掉该键的 user/runtime 覆盖, 回落到默认层.
func (s *Service) Reset(_ domaincontext.Context, name string) error {
	if _, ok := s.fields[name]; !ok {
		return s.unknownKey(name)
	}
	return s.store.Reset(name)
}

// Apply 批量写入运行态: 先整体校验, 再逐键落 runtime (避免半途失败留下部分修改).
func (s *Service) Apply(_ domaincontext.Context, patch map[string]any) error {
	for name, value := range patch {
		if err := s.validate(name, value); err != nil {
			return err
		}
	}
	for name, value := range patch {
		if err := s.store.Set(name, value); err != nil {
			return err
		}
	}
	return nil
}

// Save 一次批量落盘当前覆盖项(user + runtime), 由引擎写用户目录的 settings.json.
func (s *Service) Save(_ domaincontext.Context) error {
	return s.store.Save()
}

// Fields 暴露 schema 键表(供测试与诊断).
func (s *Service) Fields() map[string]schemakit.Field { return s.fields }

// readThemeFile 读取主题文件: 相对路径经用户目录(store)读取; 绝对路径/~ 走文件系统.
func readThemeFile(files domainstore.Store, target string) ([]byte, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(target, "file://"))
	if strings.HasPrefix(trimmed, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			trimmed = home + trimmed[1:]
		}
	}
	if !strings.HasPrefix(trimmed, "/") {
		if payload, err := storekit.Read(stdctx.Background(), files, trimmed); err == nil {
			return payload, nil
		}
	}
	return os.ReadFile(trimmed)
}

// Theme 是当前主题的完整色板(light/dark 各一套 CSS 变量)。
type Theme struct {
	ID    string            `json:"id"`
	Light map[string]string `json:"light"`
	Dark  map[string]string `json:"dark"`
}

// themeFileEntry 是 theme.json 里的一项(只取需要的字段)。
type themeFileEntry struct {
	ID    string            `json:"id"`
	Light map[string]string `json:"light"`
	Dark  map[string]string `json:"dark"`
}

// Theme 返回当前生效主题的色板: 由 theme.register 指向的文件 + theme.current 选择;
// 当前值不在文件里时选第一个(与 Options 的规则一致)。
func (s *Service) Theme(_ domaincontext.Context) (Theme, error) {
	entries, err := s.readThemes()
	if err != nil {
		return Theme{}, err
	}
	if len(entries) == 0 {
		return Theme{}, fmt.Errorf("%w: settings: no theme defined in theme file", apperr.New(apperr.CodeServiceFail))
	}
	selected := entries[0]
	if current, ok := s.read("theme.current"); ok {
		if id, _ := current.Value.(string); id != "" {
			for _, entry := range entries {
				if entry.ID == id {
					selected = entry
					break
				}
			}
		}
	}
	return Theme{ID: selected.ID, Light: selected.Light, Dark: selected.Dark}, nil
}

// readThemes 读取并解析 theme.register 指向的主题文件(读不到则回落到用户目录 theme.json).
func (s *Service) readThemes() ([]themeFileEntry, error) {
	if s.files == nil {
		return nil, fmt.Errorf("%w: settings: file store unavailable", apperr.New(apperr.CodeServiceFail))
	}
	target := targetFallback
	if register, ok := s.read("theme.register"); ok {
		if value, _ := register.Value.(string); strings.TrimSpace(value) != "" {
			target = strings.TrimSpace(value)
		}
	}
	payload, err := readThemeFile(s.files, target)
	if err != nil && target != targetFallback {
		payload, err = readThemeFile(s.files, targetFallback)
	}
	if err != nil {
		return nil, fmt.Errorf("settings: read theme file %q: %w", target, err)
	}
	var entries []themeFileEntry
	if err := json.Unmarshal(payload, &entries); err != nil {
		return nil, fmt.Errorf("settings: parse theme file %q: %w", target, err)
	}
	return entries, nil
}

// optionsTheme 是"主题列表"的选项键(前端 options 用字符串表示动态来源时传这个 key).
const (
	optionsTheme   = "theme.current.options"
	optionsFont    = "appearence.font.options"
	targetFallback = "theme.json"
)

// Options 返回某个字段的**动态候选项**(前端 options 为字符串时按该键调用).
//
// 规则: 当前值不在候选列表里时, 自动选第一个(与"总是默认选择第一个主题"一致)。
func (s *Service) Options(ctx domaincontext.Context, name string) ([]string, error) {
	switch strings.TrimSpace(name) {
	case optionsFont:
		// 字体候选 = font.json 的 builtin + system 聚合(见 service/settings/font.go)
		fonts, err := s.Fonts(ctx)
		ids := make([]string, 0, len(fonts))
		if err == nil {
			for _, font := range fonts {
				ids = append(ids, font.ID)
			}
		}
		// 配置不可读时回落到内置两项: 设置界面不应因此打不开
		if len(ids) == 0 {
			ids = []string{"inter", "system"}
		}
		return ids, nil
	case optionsTheme:
		options, err := s.themeOptions(ctx)
		if err != nil {
			return nil, err
		}
		s.ensureFirst(ctx, "theme.current", options)
		return options, nil
	default:
		return []string{}, nil
	}
}

// themeOptions 读取 theme.register 指向的 theme.json -> 主题 id 列表.
func (s *Service) themeOptions(_ domaincontext.Context) ([]string, error) {
	entries, err := s.readThemes()
	if err != nil {
		return nil, err
	}
	options := make([]string, 0, len(entries))
	for _, entry := range entries {
		if id := strings.TrimSpace(entry.ID); id != "" {
			options = append(options, id)
		}
	}
	return options, nil
}

// ensureFirst 当前值不在候选里时改为第一个(只改运行态, 由前端决定是否落盘).
func (s *Service) ensureFirst(_ domaincontext.Context, key string, options []string) {
	if len(options) == 0 {
		return
	}
	current, ok := s.read(key)
	if ok {
		if value, _ := current.Value.(string); value == options[0] {
			return
		}
		for _, option := range options {
			if value, _ := current.Value.(string); value == option {
				return
			}
		}
	}
	// 只改运行态: 是否落盘由前端在保存时决定(避免读取选项产生磁盘副作用)
	_ = s.store.Set(key, options[0])
}

// read 取生效值: 引擎优先, 其次默认值; 都缺失时返回 false.
func (s *Service) read(name string) (Entry, bool) {
	if value, ok := s.store.Get(name); ok {
		return Entry{Value: value, View: string(s.store.View(name))}, true
	}
	if value, ok := s.defaults[name]; ok {
		return Entry{Value: value, View: ViewDefault}, true
	}
	return Entry{}, false
}

// validate 按 schema 的键与类型校验取值.
func (s *Service) validate(name string, value any) error {
	field, ok := s.fields[name]
	if !ok {
		return s.unknownKey(name)
	}
	if !matches(field, value) {
		return s.typeMismatch(name, field, value)
	}
	return nil
}

func (s *Service) unknownKey(name string) error {
	return fmt.Errorf("%w: unknown settings key %q", apperr.New(apperr.CodeNotFound), name)
}

func (s *Service) typeMismatch(name string, field schemakit.Field, value any) error {
	return fmt.Errorf("%w: settings key %q expects %s, got %T",
		apperr.New(apperr.CodeServiceFail), name, describe(field), value)
}

func describe(field schemakit.Field) string {
	if field.Kind == schemakit.KindArray {
		return fmt.Sprintf("array<%s>", field.Items)
	}
	if field.Kind == schemakit.KindEnum {
		return fmt.Sprintf("enum%v", field.Enum)
	}
	return string(field.Kind)
}

// matches 判定取值是否符合字段形态: 标量同类型; enum 必须落在允许值内; array 逐元素判定。
func matches(field schemakit.Field, value any) bool {
	switch field.Kind {
	case schemakit.KindString, schemakit.KindNumber, schemakit.KindBoolean:
		return matchesScalar(string(field.Kind), value)
	case schemakit.KindEnum:
		for _, allowed := range field.Enum {
			if reflect.DeepEqual(allowed, value) {
				return true
			}
		}
		return false
	case schemakit.KindArray:
		return arrayMatches(field.Items, value)
	default:
		return false
	}
}

func arrayMatches(items schemakit.Kind, value any) bool {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice {
		return false
	}
	for i := 0; i < rv.Len(); i++ {
		if !matchesScalar(string(items), rv.Index(i).Interface()) {
			return false
		}
	}
	return true
}

// matchesScalar 判定标量类型; JSON 解出的数字统一为 float64, 同时放行整型写法。
func matchesScalar(kind string, value any) bool {
	switch kind {
	case "string":
		_, ok := value.(string)
		return ok
	case "number", "integer":
		switch value.(type) {
		case float64, float32, int, int32, int64:
			return true
		default:
			return false
		}
	case "boolean":
		_, ok := value.(bool)
		return ok
	default:
		return false
	}
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
