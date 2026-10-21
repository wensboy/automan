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
	bridgeengine "github.com/wensboy/automan/backend/engine/bridge"
	confengine "github.com/wensboy/automan/backend/engine/config"
	databaseengine "github.com/wensboy/automan/backend/engine/database"
	"github.com/wensboy/automan/backend/engine/eventbus"
	logengine "github.com/wensboy/automan/backend/engine/log"
	rtengine "github.com/wensboy/automan/backend/engine/runtime"
	settingsengine "github.com/wensboy/automan/backend/engine/settings"
	storeengine "github.com/wensboy/automan/backend/engine/store"
	winengine "github.com/wensboy/automan/backend/engine/window"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/registry"
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
			application.NewService(apiweb.NewWorkspaceService(rt)),
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

	cleanup, err := registerServices(serviceDeps{
		runtime:  rt,
		data:     data,
		fields:   fields,
		defaults: defaults,
	})
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
