// Package bootstrap 是组合根: 建运行时, 注册引擎, 装配服务与暴露层.
package bootstrap

import (
	"errors"
	"io/fs"

	"github.com/wailsapp/wails/v3/pkg/application"

	apiweb "github.com/wensboy/automan/backend/api/web"
	bridgeasset "github.com/wensboy/automan/backend/bridge"
	noderunner "github.com/wensboy/automan/backend/bridge/runner/node"
	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
	domainconfig "github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/engine"
	bridgeengine "github.com/wensboy/automan/backend/engine/bridge"
	confengine "github.com/wensboy/automan/backend/engine/config"
	databaseengine "github.com/wensboy/automan/backend/engine/database"
	"github.com/wensboy/automan/backend/engine/eventbus"
	logengine "github.com/wensboy/automan/backend/engine/log"
	rtengine "github.com/wensboy/automan/backend/engine/runtime"
	settingsengine "github.com/wensboy/automan/backend/engine/settings"
	storeengine "github.com/wensboy/automan/backend/engine/store"
	winengine "github.com/wensboy/automan/backend/engine/window"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/registry"
)

// Run 组装并执行完整启动链路.
func Run(assets, data fs.FS) error {

	rt := rtengine.New(rtengine.WithMode(context.ModeDesktop), rtengine.WithRegistry(registry.New()))
	defer rt.Cancel()

	win := winengine.New(
		winengine.WithAssets(assets),
		winengine.WithServices(
			application.NewService(apiweb.NewSettingsService(rt)),
			application.NewService(apiweb.NewProxyService(rt)),
			application.NewService(apiweb.NewSessionService(rt)),
			application.NewService(apiweb.NewUserService(rt)),
			application.NewService(apiweb.NewNoticeService(rt)),
			application.NewService(apiweb.NewLogService(rt)),
			application.NewService(apiweb.NewAnalysisService(rt)),
			application.NewService(apiweb.NewAboutService(rt)),
			application.NewService(apiweb.NewEnvironmentsService(rt)),
			application.NewService(apiweb.NewHelpService(rt)),
			application.NewService(apiweb.NewCommandService(rt)),
			application.NewService(apiweb.NewWorkspaceService(rt)),
		),
	)

	fields, err := loadSchemaFields(data)
	if err != nil {
		return err
	}

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
		// 通用桥接引擎 + daemon="node" 的运行器: 引擎不认识 node, 只按 daemon 名取运行器
		bridgeengine.New(
			bridgeengine.WithDaemon("node"),
			bridgeengine.WithRunner(func(deps domainbridge.RunnerDeps) (domainbridge.Runner, error) {
				return noderunner.New(
					noderunner.WithAsset(bridgeasset.Asset("node/dist")),
					noderunner.WithStore(deps.Store),
					noderunner.WithRooter(deps.Rooter),
					noderunner.WithLogger(deps.Logger),
				), nil
			}),
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

	cleanup, err := registerServices(serviceDeps{
		runtime:  rt,
		data:     data,
		provider: provider,
		fields:   fields,
	})
	if err != nil {
		return err
	}
	defer cleanup()

	return win.Run()
}
