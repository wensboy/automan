// Package settings 实现运行态设置引擎: 三视图叠加 + HMR + 用户文件落盘.
package settings

import (
	"errors"
	"sync"
	"time"

	"github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/event"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
	"github.com/wensboy/automan/backend/internal/fallback"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// 缺省参数, 可经 data/conf/settings.json 与用户视图覆盖.
const (
	// defaultUserFile 是用户设置文件相对存储根的路径 -> <UserConfigDir>/automan/settings.json.
	defaultUserFile   = "settings.json"
	defaultWatchMs    = 2000
	defaultDebounceMs = 100
	// saveAttempts/saveRetryDelay 约束退出路径的落盘重试, 避免下次启动读到旧值造成割裂.
	saveAttempts   = 3
	saveRetryDelay = 200 * time.Millisecond
)

// Engine 是运行态设置引擎, 实现 domain/settings.Store.
type Engine struct {
	mu     sync.Mutex // 保护停机状态
	views  *views
	notify *notifier
	stamp  fileStamp
	done   chan struct{}
	closed bool

	store  domainstore.Store
	logger domainlog.Logger
	ebus   event.Bus

	path   string
	params config.Settings // 底层类型化默认

	// persistFilter 限定可写入用户文件的键; 为空表示全部可写。
	// 用途: 用户文件只承载"用户可配置的键", 程序参数(引擎自用)与运行态数据不落进该文件。
	persistFilter func(name string) bool
}

// Option 设置设置引擎的必要字段.
type Option func(*Engine)

// WithPath 覆盖用户设置文件路径 (相对存储根, 缺省 settings.json).
func WithPath(path string) Option { return func(e *Engine) { e.path = path } }

// WithPersistFilter 限定写入用户文件的键(缺省全部可写).
// 组合根传入"用户可配置键集合", 避免程序参数/运行态键被写进用户文件。
func WithPersistFilter(allow func(name string) bool) Option {
	return func(e *Engine) { e.persistFilter = allow }
}

// New 创建设置引擎.
func New(opts ...Option) *Engine {
	e := &Engine{done: make(chan struct{})}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Name 实现 engine.Engine.
func (e *Engine) Name() string { return "settings" }

// Deps 实现 engine.Engine.
func (e *Engine) Deps() []string { return []string{"config", "eventbus", "log", "store"} }

// Init 装配底层视图并解析引擎参数.
func (e *Engine) Init(ctx context.Context) error {
	provider, ok := enginekit.Of[config.Provider](ctx, "config")
	if !ok {
		return errConfigUnavailable
	}
	store, ok := enginekit.Of[domainstore.Store](ctx, "store")
	if !ok {
		return errStoreUnavailable
	}
	logger, ok := enginekit.Of[domainlog.Logger](ctx, "log")
	if !ok {
		return errLogUnavailable
	}
	ebus, ok := enginekit.Of[event.Bus](ctx, "eventbus")
	if !ok {
		return errEventBusUnavailable
	}
	e.store, e.logger, e.ebus = store, logger, ebus
	if e.path == "" {
		e.path = defaultUserFile
	}
	bottom, err := bottomView(provider)
	if err != nil {
		return err
	}
	e.views = newViews(bottom)
	// 引擎参数的底层默认取类型化映射; 业务键保持开放 (仅按路径取值).
	e.params, _ = configkit.Get[config.Settings](provider, config.SettingsKey)
	return nil
}

// Start 读取用户视图, 解析参数并启动外部变更监听.
func (e *Engine) Start(context.Context) error {
	if err := e.loadUser(); err != nil {
		e.logError("settings: load user view failed", logkit.F("path", e.path), logkit.F("err", err))
	}
	watch := e.intParam(domainsettings.WatchMsKey.Name(), e.params.WatchMs, defaultWatchMs)
	debounce := e.intParam(domainsettings.DebounceMsKey.Name(), e.params.DebounceMs, defaultDebounceMs)

	e.notify = newNotifier(time.Duration(debounce)*time.Millisecond, e.emitChanged)
	e.notify.start()
	go e.watch(time.Duration(watch) * time.Millisecond)

	keys := e.views.count()
	e.logInfo("settings: loaded", logkit.F("keys", keys), logkit.F("path", e.path))
	e.emit(event.TopicSettingsLoaded, event.SettingsLoaded{Keys: keys})
	return nil
}

// Stop 停止监听, 末次落盘 (有界重试) 并清理视图.
func (e *Engine) Stop(context.Context) error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	close(e.done)
	e.mu.Unlock()

	e.notify.stop()
	if err := e.saveWithRetry(); err != nil {
		e.logError("settings: final save failed", logkit.F("path", e.path), logkit.F("err", err))
	}
	e.views.clear()
	return nil
}

// Get 实现 domain/settings.Store: 返回三层叠加后的生效值.
func (e *Engine) Get(name string) (any, bool) {
	value, view := e.views.get(name)
	return value, view != domainsettings.ViewNone
}

// Set 实现 domain/settings.Store: 写运行态层 (最高优先) 并广播变更.
func (e *Engine) Set(name string, v any) error {
	e.views.set(name, v)
	e.notify.push(name, v)
	return nil
}

// Reset 实现 domain/settings.Store: 移除覆盖, 回到默认.
func (e *Engine) Reset(name string) error {
	e.views.reset(name)
	if value, ok := e.Get(name); ok {
		e.notify.push(name, value)
	}
	return nil
}

// View 实现 domain/settings.Store: 返回值来自哪一层.
func (e *Engine) View(name string) domainsettings.View {
	_, view := e.views.get(name)
	return view
}

// Save 实现 domain/settings.Store: 叠加 user+runtime 回写用户文件.
func (e *Engine) Save() error { return e.save() }

// Reload 实现 domain/settings.Store: 重新读取用户视图.
func (e *Engine) Reload() error { return e.reload() }

// intParam 解析引擎参数: 三层生效值 > 底层类型化默认 > 代码缺省.
func (e *Engine) intParam(name string, typed, def int) int {
	if value, ok := e.Get(name); ok {
		if n, ok := cast.As[int](value); ok {
			return n
		}
	}
	return fallback.Int(typed, def)
}

func (e *Engine) emit(topic event.Topic, payload any) {
	if e.ebus != nil {
		e.ebus.Emit(topic, payload)
	}
}

func (e *Engine) emitChanged(name string, value any) {
	e.emit(event.TopicSettingsChanged, event.SettingsChanged{Key: name, Value: value})
}

func (e *Engine) logInfo(msg string, fields ...domainlog.Field) {
	e.logger.Deliver(logkit.New(domainlog.LevelInfo, msg, fields...))
}

func (e *Engine) logWarn(msg string, fields ...domainlog.Field) {
	e.logger.Deliver(logkit.New(domainlog.LevelWarn, msg, fields...))
}

func (e *Engine) logError(msg string, fields ...domainlog.Field) {
	e.logger.Deliver(logkit.New(domainlog.LevelError, msg, fields...))
}

var (
	errConfigUnavailable   = errors.New(`settings: engine "config" unavailable`)
	errStoreUnavailable    = errors.New(`settings: engine "store" unavailable`)
	errLogUnavailable      = errors.New(`settings: engine "log" unavailable`)
	errEventBusUnavailable = errors.New(`settings: engine "eventbus" unavailable`)
)
