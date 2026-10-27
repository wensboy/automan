// About 绑定: 程序自身信息(版本/构建/许可/仓库)。
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	serviceabout "github.com/wensboy/automan/backend/service/about"
)

// AboutService 绑定到前端。
type AboutService struct {
	ctx context.Context
}

// NewAboutService 创建绑定。
func NewAboutService(ctx context.Context) *AboutService {
	return &AboutService{ctx: ctx}
}

// Info 返回 about 页面信息(唯一数据来源=后端)。
func (s *AboutService) Info() (serviceabout.Info, error) {
	svc, ok := srvkit.Of(s.ctx, serviceabout.Key)
	if !ok {
		return serviceabout.Info{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Info(), nil
}
