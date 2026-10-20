// 工作区服务: 只做转发(列表/创建/删除/当前/切换), 业务在 service/workspace.
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	serviceworkspace "github.com/wensboy/automan/backend/service/workspace"
)

// WorkspaceService 是绑定到前端的工作区服务.
type WorkspaceService struct {
	ctx context.Context
}

// NewWorkspaceService 创建工作区服务绑定.
func NewWorkspaceService(ctx context.Context) *WorkspaceService {
	return &WorkspaceService{ctx: ctx}
}

// List 返回当前用户的工作区(没有时自动创建 Default).
func (s *WorkspaceService) List() ([]serviceworkspace.Workspace, error) {
	svc, ok := srvkit.Of(s.ctx, serviceworkspace.Key)
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound)
	}
	return svc.List(s.ctx)
}

// Create 新建工作区.
func (s *WorkspaceService) Create(name string) (serviceworkspace.Workspace, error) {
	svc, ok := srvkit.Of(s.ctx, serviceworkspace.Key)
	if !ok {
		return serviceworkspace.Workspace{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Create(s.ctx, name)
}

// Delete 删除工作区(逻辑删除; 其代理保留但不可见).
func (s *WorkspaceService) Delete(id string) error {
	svc, ok := srvkit.Of(s.ctx, serviceworkspace.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.Delete(s.ctx, id)
}

// Current 返回当前工作区 id.
func (s *WorkspaceService) Current() string {
	svc, ok := srvkit.Of(s.ctx, serviceworkspace.Key)
	if !ok {
		return ""
	}
	return svc.Current(s.ctx)
}

// SetCurrent 切换当前工作区.
func (s *WorkspaceService) SetCurrent(id string) error {
	svc, ok := srvkit.Of(s.ctx, serviceworkspace.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	return svc.SetCurrent(s.ctx, id)
}
