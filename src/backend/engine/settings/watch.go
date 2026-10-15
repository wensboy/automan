package settings

import (
	stdctx "context"
	"errors"
	"io/fs"
	"sync"
	"time"

	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
	"github.com/wensboy/automan/backend/internal/kits/storekit"
)

// fileStamp 记录用户文件的已知状态, 用于识别外部编辑并抑制自触发.
type fileStamp struct {
	mu    sync.Mutex
	mod   time.Time
	size  int64
	valid bool
}

func (s *fileStamp) set(mod time.Time, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mod, s.size, s.valid = mod, size, true
}

// changed 报告文件状态是否不同于已知状态.
func (s *fileStamp) changed(mod time.Time, size int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid {
		return true
	}
	return !mod.Equal(s.mod) || size != s.size
}

// stampNow 记录当前文件状态, 用于抑制自身写入触发的重载.
func (e *Engine) stampNow() {
	info, err := storekit.Stat(stdctx.Background(), e.store, e.path)
	if err != nil {
		return
	}
	e.stamp.set(info.ModTime, info.Size)
}

// watch 轮询用户文件, 检测到外部编辑则重载 (HMR).
func (e *Engine) watch(interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-e.done:
			return
		case <-ticker.C:
			if e.detect() {
				_ = e.reload()
			}
		}
	}
}

// detect 报告用户文件是否被外部改动.
func (e *Engine) detect() bool {
	info, err := storekit.Stat(stdctx.Background(), e.store, e.path)
	if err != nil {
		return false
	}
	return e.stamp.changed(info.ModTime, info.Size)
}

// loadUser 读取用户文件到 user 层; 文件不存在视为空视图 (首次运行).
func (e *Engine) loadUser() error {
	data, err := storekit.Read(stdctx.Background(), e.store, e.path)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		return err
	}
	next, err := decode(data)
	if err != nil {
		return err
	}
	e.views.replaceUser(next)
	e.stampNow()
	return nil
}

// reload 重新读取用户视图并广播; 失败时保持旧视图不变 (fail-safe).
func (e *Engine) reload() error {
	if err := e.loadUser(); err != nil {
		e.logWarn("settings: reload skipped", logkit.F("path", e.path), logkit.F("err", err))
		e.emit(event.TopicSettingsReloaded, event.SettingsReloaded{Err: err.Error()})
		return err
	}
	keys := e.views.count()
	e.logInfo("settings: reloaded", logkit.F("keys", keys), logkit.F("path", e.path))
	e.emit(event.TopicSettingsReloaded, event.SettingsReloaded{Keys: keys})
	return nil
}
