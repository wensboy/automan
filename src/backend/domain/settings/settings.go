// Package settings 定义运行期动态设置的领域契约与键 (读写辅助见 internal/kits/settingskit).
// 约定: 键名即该键在 settings.json 中的路径 (点号分隔).
package settings

import "github.com/wensboy/automan/backend/domain/key"

// View 表示设置值来自哪一层 (优先级 runtime > user > bottom).
type View string

const (
	ViewNone    View = ""
	ViewRuntime View = "runtime"
	ViewUser    View = "user"
	ViewBottom  View = "bottom"
)

// Store 是运行态引擎对上暴露的读写能力.
type Store interface {
	Get(name string) (any, bool)
	Set(name string, v any) error
	Save() error
	Reload() error
	Reset(name string) error
	View(name string) View
}

// 键: 名字即 settings.json 中的路径.
var (
	// WatchMsKey 外部变更检测的轮询间隔 (毫秒).
	WatchMsKey = key.New[int]("watchMs")
	// DebounceMsKey 变更事件的合并窗口 (毫秒), 非正表示不去抖.
	DebounceMsKey = key.New[int]("debounceMs")
)
