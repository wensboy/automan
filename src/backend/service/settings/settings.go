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
	"fmt"
	"reflect"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/key"
	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/schemakit"
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
	store    domainsettings.Store
	fields   map[string]schemakit.Field
	defaults map[string]any
}

// New 创建设置服务.
// fields 为 schema 解析出的键表(决定"哪些键可读写、各是什么类型"), defaults 为默认值(点号键)。
func New(store domainsettings.Store, fields map[string]schemakit.Field, defaults map[string]any) *Service {
	if defaults == nil {
		defaults = map[string]any{}
	}
	return &Service{store: store, fields: fields, defaults: defaults}
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
