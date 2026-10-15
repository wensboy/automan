// Package app 编排应用服务: 经 ereg 取引擎能力, 只依赖 domain 与上下文.
package app

import (
	"fmt"

	"github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/key"
	"github.com/wensboy/automan/backend/domain/log"
	"github.com/wensboy/automan/backend/domain/settings"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
	"github.com/wensboy/automan/backend/internal/kits/settingskit"
)

// Key 是应用服务在 srvs 中的键.
var Key = key.New[*Service]("service.app")

// Service 经上下文解析引擎完成编排, 不感知 engine 实现.
type Service struct{}

// New 创建应用服务.
func New() *Service { return &Service{} }

// Greet 读取应用元信息并记录运行态称呼.
func (s *Service) Greet(ctx context.Context, name string) (string, error) {
	provider, ok := enginekit.Of[config.Provider](ctx, "config")
	if !ok {
		return "", apperr.New(apperr.CodeServiceFail)
	}
	info, ok := configkit.Get(provider, config.ApplicationKey)
	if !ok {
		return "", apperr.New(apperr.CodeNotFound)
	}
	if store, ok := enginekit.Of[settings.Store](ctx, "settings"); ok {
		if err := settingskit.Set(store, settings.LastGreetKey, name); err != nil {
			return "", err
		}
	}
	message := fmt.Sprintf("Hello %s, from %s v%s", name, info.Name, info.Version)
	if logger, ok := enginekit.Of[log.Logger](ctx, "log"); ok {
		logger.Deliver(logkit.New(log.LevelInfo, "greet", logkit.F("name", name), logkit.F("message", message)))
	}
	return message, nil
}

// LastGreet 读取最近一次问候的称呼.
func (s *Service) LastGreet(ctx context.Context) string {
	store, ok := enginekit.Of[settings.Store](ctx, "settings")
	if !ok {
		return ""
	}
	name, _ := settingskit.Get(store, settings.LastGreetKey)
	return name
}
