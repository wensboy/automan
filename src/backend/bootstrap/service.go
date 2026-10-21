package bootstrap

import (
	"errors"
	"io/fs"

	domainconfig "github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/event"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainsettings "github.com/wensboy/automan/backend/domain/settings"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	bridgeengine "github.com/wensboy/automan/backend/engine/bridge"
	rtengine "github.com/wensboy/automan/backend/engine/runtime"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/kits/schemakit"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	servicecommand "github.com/wensboy/automan/backend/service/command"
	servicehelp "github.com/wensboy/automan/backend/service/help"
	serviceproxy "github.com/wensboy/automan/backend/service/proxy"
	servicesession "github.com/wensboy/automan/backend/service/session"
	servicesettings "github.com/wensboy/automan/backend/service/settings"
	serviceuser "github.com/wensboy/automan/backend/service/user"
	serviceworkspace "github.com/wensboy/automan/backend/service/workspace"
)

// serviceDeps 是服务装配所需的**全部输入**。
// 用结构体承载而不是长形参列表: 以后新增输入只改结构体字段, 调用方与签名都不动。
type serviceDeps struct {
	runtime  *rtengine.Runtime
	data     fs.FS
	provider domainconfig.Provider
	fields   map[string]schemakit.Field
	defaults map[string]any
}

// serviceEngines 是一次性解析出来的引擎依赖。
// 约定: **所有引擎在任何服务注册之前解析完毕**(见 docs/design/v21.md §13),
// 这样"缺引擎"会在装配早期以明确错误退出, 不会出现"注册了一半才发现缺依赖"。
type serviceEngines struct {
	settings domainsettings.Store
	db       domainstore.Store
	logger   domainlog.Logger
	ebus     event.Bus

	rooter   domainstore.Rooter
	bridge   *bridgeengine.Engine
	bridgeOK bool
}

// resolveServiceEngines 解析服务装配所需的全部引擎.
func resolveServiceEngines(rt *rtengine.Runtime) (serviceEngines, error) {
	var engines serviceEngines

	settings, ok := enginekit.Of[domainsettings.Store](rt, "settings")
	if !ok {
		return engines, errors.New("bootstrap: settings engine unavailable")
	}
	engines.settings = settings

	db, ok := enginekit.Of[domainstore.Store](rt, "database")
	if !ok {
		return engines, errors.New("bootstrap: database engine unavailable")
	}
	engines.db = db

	logger, ok := enginekit.Of[domainlog.Logger](rt, "log")
	if !ok {
		return engines, errors.New("bootstrap: log engine unavailable")
	}
	engines.logger = logger

	ebus, ok := enginekit.Of[event.Bus](rt, "eventbus")
	if !ok {
		return engines, errors.New("bootstrap: eventbus engine unavailable")
	}
	engines.ebus = ebus

	// 以下两个是可选依赖: 缺了只是"该能力不可用", 不影响其它服务
	if rooter, ok := enginekit.Of[domainstore.Rooter](rt, "store"); ok {
		engines.rooter = rooter
	}
	if bridge, ok := enginekit.Of[*bridgeengine.Engine](rt, "bridge.node"); ok {
		engines.bridge = bridge
		engines.bridgeOK = true
	}
	return engines, nil
}

// registerServices 装配全部领域服务并返回清理函数(由调用方 defer)。
// 顺序: 先解析引擎 -> 再按依赖顺序注册服务(settings -> user -> session -> workspace -> proxy -> help -> command)。
func registerServices(deps serviceDeps) (func(), error) {
	engines, err := resolveServiceEngines(deps.runtime)
	if err != nil {
		return nil, err
	}

	if err := srvkit.Register(deps.runtime, servicesettings.Key,
		servicesettings.New().WithStore(engines.settings).WithFields(deps.fields).WithDefaults(deps.defaults)); err != nil {
		return nil, err
	}

	// 用户与会话: session 直接经 settings engine 读生效配置, user 只管用户数据(两个 srv 互不调用)
	if err := srvkit.Register(deps.runtime, serviceuser.Key,
		serviceuser.New().WithStore(engines.db).WithLogger(engines.logger)); err != nil {
		return nil, err
	}
	sessionSrv := servicesession.New().
		WithStore(engines.db).
		WithSettings(engines.settings).
		WithEventBus(engines.ebus).
		WithLogger(engines.logger)
	if err := srvkit.Register(deps.runtime, servicesession.Key, sessionSrv); err != nil {
		sessionSrv.Close()
		return nil, err
	}

	// 工作区: user 1:n workspace 1:n proxy; "当前工作区"记在 workspace 服务内部
	workspaceSrv := serviceworkspace.New().
		WithStore(engines.db).
		WithLogger(engines.logger).
		WithSettings(engines.settings).
		WithCurrentUser(sessionSrv.CurrentUserID)
	if err := srvkit.Register(deps.runtime, serviceworkspace.Key, workspaceSrv); err != nil {
		return nil, err
	}

	// 代理: 模板/basedir 由服务内部经数据源读取(v19 §12); 数据按用户/工作区收束
	proxySrv := serviceproxy.New().
		WithStore(engines.db).
		WithLogger(engines.logger).
		WithData(deps.data).
		WithConfig(deps.provider)
	// 数据根: 供相对 basedir/storage 解析成绝对路径(否则会成为子进程 cwd 而导致 ENOENT)
	if engines.rooter != nil {
		proxySrv = proxySrv.WithRoot(engines.rooter.Root())
	}
	proxySrv = proxySrv.
		WithCurrentUser(sessionSrv.CurrentUserID).
		WithCurrentWorkspace(func() string { return workspaceSrv.Current(deps.runtime) })
	// 执行链路: 适配与配表逻辑都在 service/proxy 内部(见 v19 §13)
	if engines.bridgeOK {
		proxySrv = proxySrv.WithBridge(engines.bridge, engines.bridge)
	}
	// 启动期保证代理 basedir 存在(它是 node 侧 spawn 的 cwd, 也是 whistle storage 的父目录)
	if err := proxySrv.EnsureBaseDir(); err != nil {
		return nil, err
	}
	if err := srvkit.Register(deps.runtime, serviceproxy.Key, proxySrv); err != nil {
		return nil, err
	}

	if err := srvkit.Register(deps.runtime, servicehelp.Key,
		servicehelp.New().WithData(deps.data).WithLogger(engines.logger).WithEventBus(engines.ebus)); err != nil {
		return nil, err
	}

	// 命令系统: 清单来自 data/conf/command.json 的 runtime 段; 扩展命令在代码层 Register
	if err := srvkit.Register(deps.runtime, servicecommand.Key,
		servicecommand.New().WithConfig(deps.provider).WithLogger(engines.logger)); err != nil {
		return nil, err
	}

	return sessionSrv.Close, nil
}
