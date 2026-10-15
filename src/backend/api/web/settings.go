// 设置服务: 作为 web 包的一个 wails 服务暴露给前端(api/ 只放 cli/web/sdk, 域服务经 web 暴露).
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	servicesettings "github.com/wensboy/automan/backend/service/settings"
)

// SettingsService 是绑定到前端的 wails 服务, 经 srvs 解析设置服务.
type SettingsService struct {
	ctx context.Context
}

// NewSettingsService 创建前端绑定服务.
func NewSettingsService(ctx context.Context) *SettingsService {
	return &SettingsService{ctx: ctx}
}

// List 返回全部配置项的生效值与来源层.
func (s *SettingsService) List() (map[string]servicesettings.Entry, error) {
	svc, ok := srvkit.Of(s.ctx, servicesettings.Key)
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound)
	}
	return svc.List(s.ctx)
}

// Get 读取单个配置项.
func (s *SettingsService) Get(name string) (servicesettings.Entry, error) {
	svc, ok := srvkit.Of(s.ctx, servicesettings.Key)
	if !ok {
		return servicesettings.Entry{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Get(s.ctx, name)
}

// Set 写入单个配置项(运行态, 不落盘).
func (s *SettingsService) Set(name string, value any) error {
	svc, ok := srvkit.Of(s.ctx, servicesettings.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.Set(s.ctx, name, value)
}

// Apply 批量写入配置项(运行态, 不落盘).
func (s *SettingsService) Apply(patch map[string]any) error {
	svc, ok := srvkit.Of(s.ctx, servicesettings.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.Apply(s.ctx, patch)
}

// Reset 抹掉单个配置项的覆盖, 回落默认层.
func (s *SettingsService) Reset(name string) error {
	svc, ok := srvkit.Of(s.ctx, servicesettings.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.Reset(s.ctx, name)
}

// Save 落盘当前覆盖项.
func (s *SettingsService) Save() error {
	svc, ok := srvkit.Of(s.ctx, servicesettings.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.Save(s.ctx)
}
