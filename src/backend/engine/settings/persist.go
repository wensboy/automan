package settings

import (
	"bytes"
	stdctx "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/internal/cast"
	"github.com/wensboy/automan/backend/internal/flat"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
	"github.com/wensboy/automan/backend/internal/kits/storekit"
)

// bottomView 从 config 命名空间装配底层视图 (只读默认值).
func bottomView(provider config.Provider) (map[string]any, error) {
	root, ok := provider.Lookup(config.SettingsKey.Name())
	if !ok {
		return map[string]any{}, nil
	}
	tree, ok := cast.As[flat.Tree](root)
	if !ok {
		return nil, fmt.Errorf("settings: config section %q invalid", config.SettingsKey.Name())
	}
	bottom := map[string]any{}
	flat.Paths(bottom, "", tree)
	return bottom, nil
}

// decode 把用户文件内容解析为平铺键.
func decode(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var tree flat.Tree
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("settings: decode: %w", err)
	}
	paths := map[string]any{}
	flat.Paths(paths, "", tree)
	return paths, nil
}

// filterOverrides 只保留允许落盘的键; allow 为空表示不过滤.
func filterOverrides(overrides map[string]any, allow func(name string) bool) map[string]any {
	if allow == nil {
		return overrides
	}
	kept := make(map[string]any, len(overrides))
	for name, value := range overrides {
		if allow(name) {
			kept[name] = value
		}
	}
	return kept
}

// save 叠加 user+runtime 回写用户文件 (原子写), 成功后把 runtime 提升进 user 层.
func (e *Engine) save() error {
	overrides := filterOverrides(e.views.overrides(), e.persistFilter)
	data, err := json.MarshalIndent(flat.Build(overrides), "", "    ")
	if err != nil {
		return fmt.Errorf("settings: encode: %w", err)
	}
	data = append(data, '\n')
	if err := storekit.Write(stdctx.Background(), e.store, e.path, data); err != nil {
		return err
	}
	e.views.promote()
	e.stampNow()
	e.logInfo("settings: saved", logkit.F("keys", len(overrides)), logkit.F("path", e.path))
	e.emit(event.TopicSettingsSaved, event.SettingsSaved{Keys: len(overrides)})
	return nil
}

// saveWithRetry 退出路径的落盘: 有界重试, 避免下次启动读到旧值造成割裂.
func (e *Engine) saveWithRetry() error {
	var err error
	for attempt := 1; attempt <= saveAttempts; attempt++ {
		if err = e.save(); err == nil {
			return nil
		}
		if attempt < saveAttempts {
			time.Sleep(saveRetryDelay)
		}
	}
	return err
}
