// 用户服务: 作为 web 包的一个 wails 服务暴露给前端(用户 CRUD 与口令重置).
// 会话(登录/登出/恢复)在 session.go, 两者沿用不同 srv, 互不调用(见 docs/design/v18.md §11 I)。
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	serviceuser "github.com/wensboy/automan/backend/service/user"
)

// UserService 是绑定到前端的用户服务.
type UserService struct {
	ctx context.Context
}

// NewUserService 创建用户服务绑定.
func NewUserService(ctx context.Context) *UserService {
	return &UserService{ctx: ctx}
}

// List 用户列表(搜索 + 分页).
func (s *UserService) List(query serviceuser.Query) (serviceuser.Page, error) {
	svc, ok := srvkit.Of(s.ctx, serviceuser.Key)
	if !ok {
		return serviceuser.Page{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.List(s.ctx, query)
}

// Get 单个用户.
func (s *UserService) Get(id string) (serviceuser.User, error) {
	svc, ok := srvkit.Of(s.ctx, serviceuser.Key)
	if !ok {
		return serviceuser.User{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Get(s.ctx, id)
}

// Create 创建用户.
func (s *UserService) Create(input serviceuser.Input) (serviceuser.User, error) {
	svc, ok := srvkit.Of(s.ctx, serviceuser.Key)
	if !ok {
		return serviceuser.User{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Create(s.ctx, input)
}

// Update 更新用户资料(用户名改冲突时返回明确错误).
func (s *UserService) Update(id string, input serviceuser.Input) (serviceuser.User, error) {
	svc, ok := srvkit.Of(s.ctx, serviceuser.Key)
	if !ok {
		return serviceuser.User{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Update(s.ctx, id, input)
}

// Delete 逻辑删除用户.
func (s *UserService) Delete(id string) error {
	svc, ok := srvkit.Of(s.ctx, serviceuser.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.Delete(s.ctx, id)
}

// ResetPassword 管理侧重置口令(不校验旧口令).
func (s *UserService) ResetPassword(id, newPassword string) error {
	svc, ok := srvkit.Of(s.ctx, serviceuser.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.ResetPassword(s.ctx, id, newPassword)
}
