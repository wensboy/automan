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
	domainlog "github.com/wensboy/automan/backend/domain/log"
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
// Deps 声明依赖: config/eventbus 用于取配置与订阅事件; log 用于订阅日志二级总线
// (注册表按依赖拓扑排序且 Init/Start 成对执行, 漏声明会让本引擎的 Init 早于 log 的 Init,
//
//	此时 log 的 stream out 尚未创建 -> SubscribeStream 失败并中断整个启动链)。
func (e *Engine) Deps() []string { return []string{"config", "eventbus", "log"} }

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

	// 会话事件: 注册/登录/登出/到期 -> 前端 session:*
	unsubSignup := ebus.Subscribe(event.TopicSessionSignup, e.forwardSessionSignup)
	unsubLogin := ebus.Subscribe(event.TopicSessionLogin, e.forwardSessionLogin)
	unsubLogout := ebus.Subscribe(event.TopicSessionLogout, e.forwardSessionLogout)
	unsubExpired := ebus.Subscribe(event.TopicSessionExpired, e.forwardSessionExpired)

	// 日志流: 订阅日志引擎的二级总线(不经主总线, 避免高频日志卷入所有订阅者),
	// 批量转成前端事件 "log:stream" (见 docs/design/v16.md 第 3/6 节)。
	unsubLog := func() {}
	if logger, ok := enginekit.Of[domainlog.Logger](ctx, "log"); ok {
		if streamer, ok := logger.(domainlog.Streamer); ok {
			off, err := streamer.SubscribeStream(e.forwardLogStream)
			if err != nil {
				return err
			}
			unsubLog = off
		}
	}

	e.unsub = func() {
		unsubChanged()
		unsubReloaded()
		unsubLoaded()
		unsubSaved()
		unsubSignup()
		unsubLogin()
		unsubLogout()
		unsubExpired()
		unsubLog()
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

// forwardSessionSignup 转发注册事件(前端据此更新登录态).
func (e *Engine) forwardSessionSignup(payload any) {
	signup, ok := cast.As[event.SessionSignup](payload)
	if !ok || e.app == nil {
		return
	}
	e.app.Event.Emit("session:signup", signup.UserID, signup.Username, signup.At)
}

// forwardSessionLogin 转发登录事件.
func (e *Engine) forwardSessionLogin(payload any) {
	login, ok := cast.As[event.SessionLogin](payload)
	if !ok || e.app == nil {
		return
	}
	e.app.Event.Emit("session:login", login.UserID, login.Username, login.At)
}

// forwardSessionLogout 转发登出事件.
func (e *Engine) forwardSessionLogout(payload any) {
	logout, ok := cast.As[event.SessionLogout](payload)
	if !ok || e.app == nil {
		return
	}
	e.app.Event.Emit("session:logout", logout.UserID, logout.Username, logout.At)
}

// forwardSessionExpired 转发会话到期事件(ttlcache 过期钩子).
func (e *Engine) forwardSessionExpired(payload any) {
	expired, ok := cast.As[event.SessionExpired](payload)
	if !ok || e.app == nil {
		return
	}
	e.app.Event.Emit("session:expired", expired.UserID, expired.Username, expired.At)
}
