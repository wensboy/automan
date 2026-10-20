// Package bootstrap 是组合根: 建运行时, 注册引擎, 装配服务与暴露层.
package bootstrap

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/wailsapp/wails/v3/pkg/application"

	apiweb "github.com/wensboy/automan/backend/api/web"
	bridgeasset "github.com/wensboy/automan/backend/bridge"
	domainconfig "github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/engine"
	"github.com/wensboy/automan/backend/domain/event"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	bridgeengine "github.com/wensboy/automan/backend/engine/bridge"
	confengine "github.com/wensboy/automan/backend/engine/config"
	databaseengine "github.com/wensboy/automan/backend/engine/database"
	"github.com/wensboy/automan/backend/engine/eventbus"
	logengine "github.com/wensboy/automan/backend/engine/log"
	rtengine "github.com/wensboy/automan/backend/engine/runtime"
	settingsengine "github.com/wensboy/automan/backend/engine/settings"
	storeengine "github.com/wensboy/automan/backend/engine/store"
	winengine "github.com/wensboy/automan/backend/engine/window"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/kits/schemakit"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	"github.com/wensboy/automan/backend/internal/registry"
	servicecommand "github.com/wensboy/automan/backend/service/command"
	servicehelp "github.com/wensboy/automan/backend/service/help"
	serviceproxy "github.com/wensboy/automan/backend/service/proxy"
	servicesession "github.com/wensboy/automan/backend/service/session"
	servicesettings "github.com/wensboy/automan/backend/service/settings"
	serviceuser "github.com/wensboy/automan/backend/service/user"
)

// Run 组装并执行完整启动链路.
func Run(mode context.Mode, assets, data fs.FS) error {
	// 设置的键表与默认值在装配引擎前就要就绪: 键表用于限定"哪些键可落盘", 默认值注入服务层
	fields, err := loadSchemaFields(data)
	if err != nil {
		return err
	}
	defaults, err := loadDefaults(data)
	if err != nil {
		return err
	}

	rt := rtengine.New(rtengine.WithMode(mode), rtengine.WithRegistry(registry.New()))
	defer rt.Cancel()

	win := winengine.New(
		winengine.WithAssets(assets),
		winengine.WithServices(
			application.NewService(apiweb.NewSettingsService(rt)),
			application.NewService(apiweb.NewProxyService(rt)),
			application.NewService(apiweb.NewSessionService(rt)),
			application.NewService(apiweb.NewUserService(rt)),
			application.NewService(apiweb.NewNoticeService(rt)),
			application.NewService(apiweb.NewHelpService(rt)),
			application.NewService(apiweb.NewCommandService(rt)),
		),
	)

	engines := []engine.Engine{
		confengine.New(confengine.WithFS(data), confengine.WithDir("data/conf")),
		eventbus.New(),
		storeengine.New(),
		logengine.New(),
		databaseengine.New(
			databaseengine.WithMigrations(data, "data/migrate"),
		),
		// 用户文件只承载 schema 声明的键: 程序参数(conf/settings)与运行态键不写进该文件
		settingsengine.New(settingsengine.WithPersistFilter(func(name string) bool {
			_, ok := fields[name]
			return ok
		})),
		bridgeengine.New(
			bridgeengine.WithDaemon("node"),
			bridgeengine.WithAsset(bridgeasset.Asset("node/dist")),
		),
		win,
	}
	reg := rt.Registry()
	for _, e := range engines {
		if err := reg.Register(e); err != nil {
			return err
		}
	}
	if err := rt.Start(rt); err != nil {
		return err
	}
	defer func() { _ = rt.Stop(rt) }()

	provider, ok := enginekit.Of[domainconfig.Provider](rt, "config")
	if !ok {
		return errors.New("bootstrap: config engine unavailable")
	}
	if err := registerErrors(provider); err != nil {
		return err
	}

	// 用户目录: settings.json 缺失才创建; schema 每次启动覆盖(随应用更新同步)
	if err := syncUserFiles(rt, data); err != nil {
		return err
	}

	cleanup, err := registerServices(rt, data, fields, defaults)
	if err != nil {
		return err
	}
	defer cleanup()

	if mode == context.ModeCLI {
		name, _ := configkit.Get(provider, domainconfig.ApplicationNameKey)
		version, _ := configkit.Get(provider, domainconfig.ApplicationVersionKey)
		fmt.Printf("%s %s\n", name, version)
		return nil
	}
	return win.Run()
}

func registerErrors(provider domainconfig.Provider) error {
	defs, ok := configkit.Get(provider, domainconfig.ErrorKey)
	if !ok {
		return errors.New("bootstrap: error definitions unavailable")
	}
	entries := make([]apperr.Entry, 0, len(defs.Builtin)+len(defs.Service))
	for _, item := range defs.Builtin {
		entries = append(entries, apperr.Entry{Code: item.Code, Msg: item.Msg})
	}
	for _, item := range defs.Service {
		entries = append(entries, apperr.Entry{Code: item.Code, Msg: item.Msg})
	}
	return apperr.Register(entries)
}

func registerServices(rt *rtengine.Runtime, data fs.FS, fields map[string]schemakit.Field, defaults map[string]any) (func(), error) {
	// 设置服务内部持有引擎: 组合根在此解析一次并注入
	store, ok := enginekit.Of[domainsettings.Store](rt, "settings")
	if !ok {
		return nil, errors.New("bootstrap: settings engine unavailable")
	}
	if err := srvkit.Register(rt, servicesettings.Key, servicesettings.New(store, fields, defaults)); err != nil {
		return nil, err
	}

	// 代理数据链路: 走 database 引擎执行 SQL, 经 log 引擎记录操作日志
	db, ok := enginekit.Of[domainstore.Store](rt, "database")
	if !ok {
		return nil, errors.New("bootstrap: database engine unavailable")
	}
	logger, ok := enginekit.Of[domainlog.Logger](rt, "log")
	if !ok {
		return nil, errors.New("bootstrap: log engine unavailable")
	}
	// 代理启动链路: 只把只读数据源与执行链路适配器递进服务;
	// 模板/basedir 由 service 内部经数据源读取(v19 §12: 能用内部完成的不要堆在组合根)
	proxyOpts := []serviceproxy.Option{serviceproxy.WithData(data)}
	// 数据根: 供相对 basedir/storage 解析成绝对路径(否则会成为子进程 cwd 而导致 ENOENT)
	if rooter, ok := enginekit.Of[domainstore.Rooter](rt, "store"); ok {
		proxyOpts = append(proxyOpts, serviceproxy.WithRoot(rooter.Root()))
	}
	// 执行链路的引擎在此解析; 适配与配表逻辑都在 service/proxy 内部(见 v19 §13)
	if bridgeEngine, ok := enginekit.Of[*bridgeengine.Engine](rt, "bridge.node"); ok {
		proxyOpts = append(proxyOpts, serviceproxy.WithBridge(bridgeEngine, bridgeEngine))
	}
	proxySrv := serviceproxy.New(db, logger, proxyOpts...)
	// 启动期保证代理 basedir 存在(它是 node 侧 spawn 的 cwd, 也是 whistle storage 的父目录)
	if err := proxySrv.EnsureBaseDir(); err != nil {
		return nil, err
	}
	if err := srvkit.Register(rt, serviceproxy.Key, proxySrv); err != nil {
		return nil, err
	}

	// 会话与用户: session 直接经 settings engine 读生效配置, user 只管用户数据(两个 srv 互不调用)
	settingsEngine, ok := enginekit.Of[domainsettings.Store](rt, "settings")
	if !ok {
		return nil, errors.New("bootstrap: settings engine unavailable")
	}
	ebus, ok := enginekit.Of[event.Bus](rt, "eventbus")
	if !ok {
		return nil, errors.New("bootstrap: eventbus engine unavailable")
	}
	if err := srvkit.Register(rt, serviceuser.Key, serviceuser.New(db, logger)); err != nil {
		return nil, err
	}
	sessionSrv := servicesession.New(db, settingsEngine, ebus, logger)
	if err := srvkit.Register(rt, servicesession.Key, sessionSrv); err != nil {
		sessionSrv.Close()
		return nil, err
	}
	if err := srvkit.Register(rt, servicehelp.Key, servicehelp.New(data, logger, ebus)); err != nil {
		return nil, err
	}

	// 命令系统: 清单来自 data/conf/command.json 的 runtime 段; 扩展命令在代码层 Register
	if err := srvkit.Register(rt, servicecommand.Key, servicecommand.New(data, logger)); err != nil {
		return nil, err
	}
	return sessionSrv.Close, nil
}
