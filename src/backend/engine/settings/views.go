package settings

import (
	"maps"
	"sync"

	domainsettings "github.com/wensboy/automan/backend/domain/settings"
)

// views 是三视图叠加: 优先级 runtime > user > bottom.
type views struct {
	mu      sync.RWMutex
	bottom  map[string]any
	user    map[string]any
	runtime map[string]any
}

func newViews(bottom map[string]any) *views {
	return &views{bottom: bottom, user: map[string]any{}, runtime: map[string]any{}}
}

// get 解析生效值与来源层.
func (v *views) get(name string) (any, domainsettings.View) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if value, ok := v.runtime[name]; ok {
		return value, domainsettings.ViewRuntime
	}
	if value, ok := v.user[name]; ok {
		return value, domainsettings.ViewUser
	}
	if value, ok := v.bottom[name]; ok {
		return value, domainsettings.ViewBottom
	}
	return nil, domainsettings.ViewNone
}

// set 写入 runtime 层 (最高优先).
func (v *views) set(name string, value any) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.runtime[name] = value
}

// reset 移除 runtime 与 user 层的覆盖.
func (v *views) reset(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.runtime, name)
	delete(v.user, name)
}

// replaceUser 整体替换 user 层 (外部重载).
func (v *views) replaceUser(next map[string]any) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.user = next
}

// overrides 返回 user + runtime 的叠加结果 (runtime 优先), 用于落盘.
func (v *views) overrides() map[string]any {
	v.mu.RLock()
	defer v.mu.RUnlock()
	merged := make(map[string]any, len(v.user)+len(v.runtime))
	maps.Copy(merged, v.user)
	maps.Copy(merged, v.runtime)
	return merged
}

// promote 落盘成功后调用: 把 runtime 并入 user 层并清空 runtime,
// 避免再从文件装载一次 (runtime 与 user 内容已一致).
func (v *views) promote() {
	v.mu.Lock()
	defer v.mu.Unlock()
	for name, value := range v.runtime {
		v.user[name] = value
	}
	v.runtime = map[string]any{}
}

// count 返回三层键的并集大小.
func (v *views) count() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	seen := make(map[string]struct{}, len(v.bottom)+len(v.user)+len(v.runtime))
	for _, layer := range []map[string]any{v.bottom, v.user, v.runtime} {
		for name := range layer {
			seen[name] = struct{}{}
		}
	}
	return len(seen)
}

// clear 清空内存视图.
func (v *views) clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.bottom = map[string]any{}
	v.user = map[string]any{}
	v.runtime = map[string]any{}
}
