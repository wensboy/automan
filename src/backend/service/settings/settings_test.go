package settings

import (
	"strings"
	"testing"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	"github.com/wensboy/automan/backend/internal/kits/schemakit"
)

// fakeStore 是内存版 settings.Store, 复刻引擎的三视图优先级与 promote 语义, 但不触盘。
type fakeStore struct {
	bottom  map[string]any
	user    map[string]any
	runtime map[string]any
	saves   int
}

func newFakeStore(bottom map[string]any) *fakeStore {
	return &fakeStore{bottom: bottom, user: map[string]any{}, runtime: map[string]any{}}
}

func (s *fakeStore) Get(name string) (any, bool) {
	if v, ok := s.runtime[name]; ok {
		return v, true
	}
	if v, ok := s.user[name]; ok {
		return v, true
	}
	v, ok := s.bottom[name]
	return v, ok
}

func (s *fakeStore) Set(name string, v any) error { s.runtime[name] = v; return nil }

func (s *fakeStore) Save() error {
	s.saves++
	for name, v := range s.runtime {
		s.user[name] = v
	}
	s.runtime = map[string]any{}
	return nil
}

func (s *fakeStore) Reload() error { return nil }

func (s *fakeStore) Reset(name string) error {
	delete(s.runtime, name)
	delete(s.user, name)
	return nil
}

func (s *fakeStore) View(name string) domainsettings.View {
	if _, ok := s.runtime[name]; ok {
		return domainsettings.ViewRuntime
	}
	if _, ok := s.user[name]; ok {
		return domainsettings.ViewUser
	}
	if _, ok := s.bottom[name]; ok {
		return domainsettings.ViewBottom
	}
	return domainsettings.ViewNone
}

// 与 data/schema/settings.user.schema.json 对应的最小键表。
func schemaFields() map[string]schemakit.Field {
	return map[string]schemakit.Field{
		"appearence.mode":    {Kind: schemakit.KindEnum, Enum: []any{"auto", "light", "dark"}},
		"appearence.compact": {Kind: schemakit.KindBoolean},
		"general.language":   {Kind: schemakit.KindEnum, Enum: []any{"English", "Chinese"}},
		"proxy.timeout":      {Kind: schemakit.KindNumber},
		"proxy.bypass":       {Kind: schemakit.KindArray, Items: schemakit.KindString},
		"automation.paths":   {Kind: schemakit.KindArray, Items: schemakit.KindString},
		// schema 里允许但当前没有默认值的键: 用于验证"引擎无值 + 无默认 -> 不出现在列表里"
		"general.startupPage": {Kind: schemakit.KindString},
	}
}

func newService() (*Service, *fakeStore) {
	defaults := map[string]any{
		"appearence.mode":    "auto",
		"appearence.compact": false,
		"general.language":   "English",
		"proxy.timeout":      float64(30),
		"proxy.bypass":       []any{"localhost"},
	}
	store := newFakeStore(map[string]any{})
	return New().WithStore(store).WithFields(schemaFields()).WithDefaults(defaults), store
}

// 列表: 引擎有值取引擎值(带来源层), 否则回落默认值; 都缺失的键不出现。
func TestListMergesEngineAndDefaults(t *testing.T) {
	svc, store := newService()
	store.user["general.language"] = "Chinese"

	list, err := svc.List(nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := list["appearence.mode"]; got.Value != "auto" || got.View != ViewDefault {
		t.Fatalf("默认值回落异常: %+v", got)
	}
	if got := list["general.language"]; got.Value != "Chinese" || got.View != string(domainsettings.ViewUser) {
		t.Fatalf("引擎值优先异常: %+v", got)
	}
	if _, ok := list["general.startupPage"]; ok {
		t.Fatalf("无值且无默认的键不应出现在列表里")
	}
	if _, ok := list["watchMs"]; ok {
		t.Fatalf("schema 未声明的键不应出现在列表里")
	}
}

// 键与类型校验: 未知键 / 类型不符 / 枚举越界都要被拒绝。
func TestSetValidatesAgainstSchema(t *testing.T) {
	svc, _ := newService()

	if err := svc.Set(nil, "nope.key", "x"); err == nil || !strings.Contains(err.Error(), "unknown settings key") {
		t.Fatalf("未知键应被拒绝, got %v", err)
	}
	if err := svc.Set(nil, "appearence.compact", "yes"); err == nil || !strings.Contains(err.Error(), "expects") {
		t.Fatalf("类型不符应被拒绝, got %v", err)
	}
	if err := svc.Set(nil, "appearence.mode", "blue"); err == nil || !strings.Contains(err.Error(), "expects") {
		t.Fatalf("枚举越界应被拒绝, got %v", err)
	}
	if err := svc.Set(nil, "appearence.mode", "light"); err != nil {
		t.Fatalf("合法写入失败: %v", err)
	}
	entry, err := svc.Get(nil, "appearence.mode")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.Value != "light" || entry.View != string(domainsettings.ViewRuntime) {
		t.Fatalf("写入后应落在 runtime 层, got %+v", entry)
	}
}

// 数组: 元素类型必须与 schema 的 items 一致。
func TestArrayItemsValidated(t *testing.T) {
	svc, _ := newService()

	if err := svc.Set(nil, "proxy.bypass", []string{"a", "b"}); err != nil {
		t.Fatalf("字符串数组应被接受: %v", err)
	}
	if err := svc.Set(nil, "proxy.bypass", []any{"a", 1}); err == nil {
		t.Fatalf("元素类型不符应被拒绝")
	}
	if err := svc.Set(nil, "automation.paths", []any{"p1"}); err != nil {
		t.Fatalf("空数组键接受字符串数组: %v", err)
	}
}

// 批量写入是原子的: 任一键非法则整体不生效。
func TestApplyIsAtomic(t *testing.T) {
	svc, _ := newService()

	err := svc.Apply(nil, map[string]any{
		"appearence.mode":  "dark",
		"general.language": 1,
	})
	if err == nil {
		t.Fatalf("含非法键的批量写入应失败")
	}
	entry, _ := svc.Get(nil, "appearence.mode")
	if entry.Value != "auto" {
		t.Fatalf("失败后不应留下部分修改, got %+v", entry)
	}
}

// 落盘: runtime 并入 user, 来源层随之为 user。
func TestApplyThenSavePromotes(t *testing.T) {
	svc, store := newService()

	if err := svc.Apply(nil, map[string]any{"appearence.mode": "dark", "general.language": "Chinese"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := svc.Save(nil); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if store.saves != 1 {
		t.Fatalf("Save 调用次数 want=1 got=%d", store.saves)
	}
	if len(store.runtime) != 0 {
		t.Fatalf("落盘后 runtime 应清空, got %v", store.runtime)
	}
	entry, _ := svc.Get(nil, "appearence.mode")
	if entry.Value != "dark" || entry.View != string(domainsettings.ViewUser) {
		t.Fatalf("落盘后来源层应为 user, got %+v", entry)
	}
}

// 重置: 抹掉覆盖后回落默认值。
func TestResetFallsBackToDefault(t *testing.T) {
	svc, store := newService()
	store.user["appearence.mode"] = "light"

	if err := svc.Reset(nil, "appearence.mode"); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	entry, _ := svc.Get(nil, "appearence.mode")
	if entry.Value != "auto" || entry.View != ViewDefault {
		t.Fatalf("重置后应回落默认, got %+v", entry)
	}
}

// 未落盘的运行态值只存在于 runtime 层(与 "Apply 不落盘" 的约定一致)。
func TestApplyDoesNotPersist(t *testing.T) {
	svc, store := newService()
	if err := svc.Apply(nil, map[string]any{"appearence.mode": "dark"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if store.saves != 0 || len(store.user) != 0 {
		t.Fatalf("Apply 不应落盘, saves=%d user=%v", store.saves, store.user)
	}
}

var _ = domaincontext.ModeCLI // 保持 domaincontext 引用(签名使用其类型)
