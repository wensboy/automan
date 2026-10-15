// Package window 实现表现层引擎: 承载 wails 应用与窗口, 仅 desktop 模式建窗.
package window

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/internal/cast"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
)

// Engine 持有 wails 应用实例.
type Engine struct {
	assets   fs.FS
	services []application.Service

	app   *application.App
	unsub func()
}

// Option 设置窗口引擎的必要字段.
type Option func(*Engine)

// WithAssets 指定前端资源文件系统.
func WithAssets(fsys fs.FS) Option { return func(e *Engine) { e.assets = fsys } }

// WithServices 指定暴露给前端的 wails 服务.
func WithServices(services ...application.Service) Option {
	return func(e *Engine) { e.services = services }
}

// New 创建窗口引擎.
func New(opts ...Option) *Engine {
	e := &Engine{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Name 实现 engine.Engine.
func (e *Engine) Name() string { return "window" }

// Deps 实现 engine.Engine.
func (e *Engine) Deps() []string { return []string{"config", "eventbus"} }

// Init 订阅运行态变更与外部重载, 待窗口就绪后转发给前端.
func (e *Engine) Init(ctx context.Context) error {
	ebus, ok := enginekit.Of[event.Bus](ctx, "eventbus")
	if !ok {
		return errEventBusUnavailable
	}
	unsubChanged := ebus.Subscribe(event.TopicSettingsChanged, e.forwardSettingsChanged)
	unsubReloaded := ebus.Subscribe(event.TopicSettingsReloaded, e.forwardSettingsReloaded)
	unsubLoaded := ebus.Subscribe(event.TopicSettingsLoaded, e.forwardSettingsLoaded)
	unsubSaved := ebus.Subscribe(event.TopicSettingsSaved, e.forwardSettingsSaved)
	e.unsub = func() {
		unsubChanged()
		unsubReloaded()
		unsubLoaded()
		unsubSaved()
	}
	return nil
}

// Start 在 desktop 模式下按 config 引擎的应用信息构建 wails 应用与窗口.
func (e *Engine) Start(ctx context.Context) error {
	if ctx.Mode() != context.ModeDesktop {
		return nil
	}
	provider, ok := enginekit.Of[config.Provider](ctx, "config")
	if !ok {
		return errConfigUnavailable
	}
	info, ok := configkit.Get(provider, config.ApplicationKey)
	if !ok {
		return fmt.Errorf("window: config section %q missing", config.ApplicationKey.Name())
	}
	e.app = application.New(application.Options{
		Name:        info.Name,
		Description: info.Description,
		Services:    e.services,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(e.assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	e.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  info.Name,
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})
	return nil
}

// Run 阻塞运行 wails 事件循环, 非 desktop 模式无实例时直接返回.
func (e *Engine) Run() error {
	if e.app == nil {
		return nil
	}
	return e.app.Run()
}

// Stop 仅注销订阅: 停机发生在 Run 返回之后, wails 已于最后一个窗口关闭时自行 destroy,
// 再调 app.Quit 会对已销毁的 GApplication 二次 quit (GLib assertion).
func (e *Engine) Stop(context.Context) error {
	if e.unsub != nil {
		e.unsub()
	}
	return nil
}

func (e *Engine) forwardSettingsChanged(payload any) {
	changed, ok := cast.As[event.SettingsChanged](payload)
	if !ok || e.app == nil {
		return
	}
	e.app.Event.Emit("settings:changed", changed.Key, changed.Value)
}

func (e *Engine) forwardSettingsReloaded(payload any) {
	reloaded, ok := cast.As[event.SettingsReloaded](payload)
	if !ok || e.app == nil {
		return
	}
	e.app.Event.Emit("settings:reloaded", reloaded.Keys, reloaded.Err)
}

// forwardSettingsLoaded 转发"设置装配完成"(前端可用于首次拉取后校正界面).
func (e *Engine) forwardSettingsLoaded(payload any) {
	loaded, ok := cast.As[event.SettingsLoaded](payload)
	if !ok || e.app == nil {
		return
	}
	e.app.Event.Emit("settings:loaded", loaded.Keys)
}

// forwardSettingsSaved 转发"落盘完成"(前端据此确认修改已持久化).
func (e *Engine) forwardSettingsSaved(payload any) {
	saved, ok := cast.As[event.SettingsSaved](payload)
	if !ok || e.app == nil {
		return
	}
	e.app.Event.Emit("settings:saved", saved.Keys)
}

var (
	errConfigUnavailable   = errors.New(`window: engine "config" unavailable`)
	errEventBusUnavailable = errors.New(`window: engine "eventbus" unavailable`)
)
