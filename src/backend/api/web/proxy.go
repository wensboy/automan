// 代理数据服务: 作为 web 包的一个 wails 服务暴露给前端(api/ 只放 cli/web/sdk, 域服务经 web 暴露).
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	serviceproxy "github.com/wensboy/automan/backend/service/proxy"
)

// ProxyService 是绑定到前端的代理服务, 经 srvs 解析 service/proxy.
type ProxyService struct {
	ctx context.Context
}

// NewProxyService 创建前端绑定服务.
func NewProxyService(ctx context.Context) *ProxyService {
	return &ProxyService{ctx: ctx}
}

// List 返回满足条件的当前页与总数.
func (s *ProxyService) List(query serviceproxy.Query) (serviceproxy.Page, error) {
	svc, ok := srvkit.Of(s.ctx, serviceproxy.Key)
	if !ok {
		return serviceproxy.Page{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.List(s.ctx, query)
}

// Get 读取单条代理.
func (s *ProxyService) Get(id string) (serviceproxy.Proxy, error) {
	svc, ok := srvkit.Of(s.ctx, serviceproxy.Key)
	if !ok {
		return serviceproxy.Proxy{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Get(s.ctx, id)
}

// Create 创建代理.
func (s *ProxyService) Create(input serviceproxy.Input) (serviceproxy.Proxy, error) {
	svc, ok := srvkit.Of(s.ctx, serviceproxy.Key)
	if !ok {
		return serviceproxy.Proxy{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Create(s.ctx, input)
}

// Update 更新代理.
func (s *ProxyService) Update(id string, input serviceproxy.Input) (serviceproxy.Proxy, error) {
	svc, ok := srvkit.Of(s.ctx, serviceproxy.Key)
	if !ok {
		return serviceproxy.Proxy{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Update(s.ctx, id, input)
}

// Running 返回可用于 capture 页的代理实例(仅运行中且配置了 uiport 的).
func (s *ProxyService) Running() ([]serviceproxy.CaptureTarget, error) {
	svc, ok := srvkit.Of(s.ctx, serviceproxy.Key)
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound)
	}
	return svc.Running(s.ctx)
}

// Template 返回 rules 模板(前端编辑器初始化用).
func (s *ProxyService) Template() (string, error) {
	svc, ok := srvkit.Of(s.ctx, serviceproxy.Key)
	if !ok {
		return "", apperr.New(apperr.CodeNotFound)
	}
	return svc.Template(s.ctx), nil
}

// Start 批量启动代理(参数取自 sqlite, 让步后写回; 返回实际生效值)。
func (s *ProxyService) Start(ids []string) (serviceproxy.StartResult, error) {
	svc, ok := srvkit.Of(s.ctx, serviceproxy.Key)
	if !ok {
		return serviceproxy.StartResult{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Start(s.ctx, ids)
}

// Stop 批量停止代理(进程收尾由 node runtime 兜底)。
func (s *ProxyService) Stop(ids []string) error {
	svc, ok := srvkit.Of(s.ctx, serviceproxy.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.Stop(s.ctx, ids)
}

// SetStatus 批量改状态(工具栏启动/停止): 只改选中的、未删除且状态不同的行, 返回实际改动条数.
func (s *ProxyService) SetStatus(ids []string, status string) (int, error) {
	svc, ok := srvkit.Of(s.ctx, serviceproxy.Key)
	if !ok {
		return 0, apperr.New(apperr.CodeNotFound)
	}
	return svc.SetStatus(s.ctx, ids, status)
}

// Delete 逻辑删除代理.
func (s *ProxyService) Delete(id string) error {
	svc, ok := srvkit.Of(s.ctx, serviceproxy.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.Delete(s.ctx, id)
}
