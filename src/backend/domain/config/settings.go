package config

import "github.com/wensboy/automan/backend/domain/key"

// Settings 映射 data/conf/settings.json 中"引擎自身参数"部分; 业务键保持开放.
type Settings struct {
	WatchMs    int `json:"watchMs"`
	DebounceMs int `json:"debounceMs"`
}

// SettingsKey 取 settings 默认视图的 namespace 根 (也用于类型化读取引擎参数).
var SettingsKey = key.New[Settings]("conf/settings")
