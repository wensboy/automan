// Package window 将服务编排结果以 wails 服务形式暴露给前端.
package window

import (
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	"github.com/wensboy/automan/backend/service/app"
)

// GreetService 是绑定到前端的 wails 服务, 经 srvs 解析应用服务.
type GreetService struct {
	ctx context.Context
}

// NewGreetService 创建前端绑定服务.
func NewGreetService(ctx context.Context) *GreetService { return &GreetService{ctx: ctx} }

// Greet 前端入口: 解析服务能力后转发.
func (s *GreetService) Greet(name string) (string, error) {
	svc, ok := srvkit.Of(s.ctx, app.Key)
	if !ok {
		return "", apperr.New(apperr.CodeNotFound)
	}
	return svc.Greet(s.ctx, name)
}

// LastGreet 暴露最近一次运行态数据.
func (s *GreetService) LastGreet() string {
	svc, ok := srvkit.Of(s.ctx, app.Key)
	if !ok {
		return ""
	}
	return svc.LastGreet(s.ctx)
}
