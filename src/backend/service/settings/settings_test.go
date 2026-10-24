package settings

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	domainstore "github.com/wensboy/automan/backend/domain/store"
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

// fakeFiles 是 domainstore.Store 的最小替身: 只支持读取文件.
type fakeFiles struct{ files map[string][]byte }

func (f *fakeFiles) Submit(domainstore.Task) <-chan domainstore.Result { return nil }

func (f *fakeFiles) Do(_ stdctx.Context, task domainstore.Task) (any, error) {
	if task.Cmd != domainstore.CmdRead {
		return nil, errors.New("unsupported")
	}
	args, ok := task.Args.(domainstore.ReadArgs)
	if !ok {
		return nil, errors.New("bad args")
	}
	data, exists := f.files[args.Path]
	if !exists {
		return nil, errors.New("not exist")
	}
	return data, nil
}

// 动态选项: theme.current.options 取自 theme.register 指向的文件; 当前值不在列表时默认选第一个.
func TestOptionsThemeList(t *testing.T) {
	store := newFakeStore(map[string]any{
		"theme.register": "theme.json",
		"theme.current":  "missing-theme",
	})
	files := &fakeFiles{files: map[string][]byte{
		"theme.json": []byte(`[{"id":"default"},{"id":"dark-blue"}]`),
	}}
	svc := New().WithStore(store).WithFiles(files)

	options, err := svc.Options(nil, "theme.current.options")
	if err != nil {
		t.Fatalf("Options: %v", err)
	}
	if len(options) != 2 || options[0] != "default" || options[1] != "dark-blue" {
		t.Fatalf("选项应为文件中的主题 id, got %+v", options)
	}
	if got, _ := store.Get("theme.current"); got != "default" {
		t.Fatalf("当前值不在列表时应默认第一个, got %v", got)
	}

	// 未知的选项键: 返回空列表(不报错)
	if got, err := svc.Options(nil, "unknown.key"); err != nil || len(got) != 0 {
		t.Fatalf("未知键应返回空, got %v err=%v", got, err)
	}
}

// 主题色板: 按 theme.current 选择; 当前值失效时选第一个; 返回 light/dark 两套变量.
func TestThemeSelectsCurrent(t *testing.T) {
	store := newFakeStore(map[string]any{
		"theme.register": "theme.json",
		"theme.current":  "tokyo-night",
	})
	files := &fakeFiles{files: map[string][]byte{
		"theme.json": []byte(`[
			{"id":"default","light":{"--background":"#fff"},"dark":{"--background":"#000"}},
			{"id":"tokyo-night","light":{"--background":"#e6e7ed"},"dark":{"--background":"#1a1b26"}}
		]`),
	}}
	svc := New().WithStore(store).WithFiles(files)

	theme, err := svc.Theme(nil)
	if err != nil {
		t.Fatalf("Theme: %v", err)
	}
	if theme.ID != "tokyo-night" || theme.Dark["--background"] != "#1a1b26" {
		t.Fatalf("应按 theme.current 选择, got %+v", theme)
	}

	// 当前值失效 -> 第一个
	store.runtime["theme.current"] = "missing"
	fallback, err := svc.Theme(nil)
	if err != nil {
		t.Fatalf("Theme(fallback): %v", err)
	}
	if fallback.ID != "default" {
		t.Fatalf("失效值应回落到第一个主题, got %q", fallback.ID)
	}
}

// 字体候选由后端提供(前端 options 用字符串键时按该键调用)。
func TestOptionsFonts(t *testing.T) {
	svc := New()
	got, err := svc.Options(nil, "appearence.font.options")
	if err != nil {
		t.Fatalf("Options(font): %v", err)
	}
	if len(got) != 2 || got[0] != "inter" || got[1] != "system" {
		t.Fatalf("字体候选异常: %+v", got)
	}
}

// 字体: 用户文件缺失时回落到内嵌缺省; 聚合 builtin + system 且可按 id 取 path。
func TestFontsFromEmbeddedDefault(t *testing.T) {
	data := fstest.MapFS{
		"data/default/font.json": &fstest.MapFile{Data: []byte(`{
			"builtin": [{"id":"inter","path":"\"Inter Variable\", sans-serif"}],
			"system":  [{"id":"pingfang","path":"\"PingFang SC\", sans-serif"}]
		}`)},
	}
	svc := New().WithData(data)

	fonts, err := svc.Fonts(nil)
	if err != nil {
		t.Fatalf("Fonts: %v", err)
	}
	if len(fonts) != 2 || fonts[0].ID != "inter" || fonts[1].Group != "system" {
		t.Fatalf("聚合结果异常: %+v", fonts)
	}

	font, err := svc.Font(nil, "pingfang")
	if err != nil || font.Path != `"PingFang SC", sans-serif` {
		t.Fatalf("按 id 取字体异常: %+v err=%v", font, err)
	}
	if _, err := svc.Font(nil, "missing"); err == nil {
		t.Fatal("未知 id 应报错")
	}
}
