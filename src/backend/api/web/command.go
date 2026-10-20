// 命令服务: 前端 search 的唯一入口. 只把命令 id 转给 service/command, 不承载路由逻辑.
// 清单来自 data/conf/command.json 的 runtime 段; cli 段是程序 CLI 的解析配置, 与本链路无关。
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	servicecommand "github.com/wensboy/automan/backend/service/command"
)

// CommandService 是绑定到前端的命令服务.
type CommandService struct {
	ctx context.Context
}

// NewCommandService 创建命令服务绑定.
func NewCommandService(ctx context.Context) *CommandService {
	return &CommandService{ctx: ctx}
}

// List 返回 runtime 段的命令分组(前端 search 的唯一数据源).
func (s *CommandService) List() ([]servicecommand.Group, error) {
	svc, ok := srvkit.Of(s.ctx, servicecommand.Key)
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound)
	}
	return svc.List(s.ctx)
}

// Run 执行一条命令("<group>.<cmd>"); 返回的 Action 由前端决定 UI 变化.
func (s *CommandService) Run(id string) (servicecommand.Result, error) {
	svc, ok := srvkit.Of(s.ctx, servicecommand.Key)
	if !ok {
		return servicecommand.Result{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Run(s.ctx, id)
}
