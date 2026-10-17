// 会话服务: 作为 web 包的一个 wails 服务暴露给前端(注册/登录/登出/恢复/校验/改密).
// 用户 CRUD 在 user.go(独立服务, 独立 srv)。
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	servicesession "github.com/wensboy/automan/backend/service/session"
	serviceuser "github.com/wensboy/automan/backend/service/user"
)

// SessionService 是绑定到前端的会话服务.
type SessionService struct {
	ctx context.Context
}

// NewSessionService 创建会话服务绑定.
func NewSessionService(ctx context.Context) *SessionService {
	return &SessionService{ctx: ctx}
}

// SignUp 注册并立即登录.
func (s *SessionService) SignUp(input servicesession.SignUpInput) (servicesession.Session, error) {
	svc, ok := srvkit.Of(s.ctx, servicesession.Key)
	if !ok {
		return servicesession.Session{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.SignUp(s.ctx, input)
}

// SignIn 登录.
func (s *SessionService) SignIn(username, password string) (servicesession.Session, error) {
	svc, ok := srvkit.Of(s.ctx, servicesession.Key)
	if !ok {
		return servicesession.Session{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.SignIn(s.ctx, username, password)
}

// SignOut 登出.
func (s *SessionService) SignOut(token string) error {
	svc, ok := srvkit.Of(s.ctx, servicesession.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.SignOut(s.ctx, token)
}

// Resume 启动恢复会话.
func (s *SessionService) Resume(token string) (servicesession.Session, error) {
	svc, ok := srvkit.Of(s.ctx, servicesession.Key)
	if !ok {
		return servicesession.Session{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Resume(s.ctx, token)
}

// Current 校验会话并返回当前用户.
func (s *SessionService) Current(token string) (serviceuser.User, error) {
	svc, ok := srvkit.Of(s.ctx, servicesession.Key)
	if !ok {
		return serviceuser.User{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Current(s.ctx, token)
}

// ChangePassword 修改当前会话用户的口令(成功后该用户所有会话失效).
func (s *SessionService) ChangePassword(token, oldPassword, newPassword string) error {
	svc, ok := srvkit.Of(s.ctx, servicesession.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.ChangePassword(s.ctx, token, oldPassword, newPassword)
}
