// Package bootstrap 是组合根: 建运行时, 注册引擎, 装配服务与暴露层.
package bootstrap

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/wailsapp/wails/v3/pkg/application"

	apiwindow "github.com/wensboy/automan/backend/api/window"
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
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	"github.com/wensboy/automan/backend/internal/registry"
	serviceapp "github.com/wensboy/automan/backend/service/app"
)

// Run 组装并执行完整启动链路.
func Run(mode context.Mode, assets, data fs.FS) error {
	rt := rtengine.New(rtengine.WithMode(mode), rtengine.WithRegistry(registry.New()))
	defer rt.Cancel()

	win := winengine.New(
		winengine.WithAssets(assets),
		winengine.WithServices(
			application.NewService(apiwindow.NewGreetService(rt)),
		),
	)

	engines := []engine.Engine{
		confengine.New(confengine.WithFS(data), confengine.WithDir("data/conf")),
		eventbus.New(),
		storeengine.New(),
		logengine.New(),
		databaseengine.New(),
		settingsengine.New(),
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

	if err := registerServices(rt); err != nil {
		return err
	}

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

func registerServices(rt *rtengine.Runtime) error {
	if err := srvkit.Register(rt, serviceapp.Key, serviceapp.New()); err != nil {
		return err
	}
	return nil
}
