// 帮助服务: 把离线文档(嵌入的 data/docs)暴露给前端.
// 说明: 只读; 网络获取(remote 三个 uri)当前不实现, 结构样例见 data/docs/meta.json 的 remote 段。
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	servicehelp "github.com/wensboy/automan/backend/service/help"
)

// HelpService 是绑定到前端的帮助服务.
type HelpService struct {
	ctx context.Context
}

// NewHelpService 创建帮助服务绑定.
func NewHelpService(ctx context.Context) *HelpService {
	return &HelpService{ctx: ctx}
}

// Meta 返回文档元数据(目录/版本/远程结构样例).
func (s *HelpService) Meta() (servicehelp.Meta, error) {
	svc, ok := srvkit.Of(s.ctx, servicehelp.Key)
	if !ok {
		return servicehelp.Meta{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Meta(s.ctx)
}

// Doc 返回一篇 markdown 原文(前端用 marked 渲染).
func (s *HelpService) Doc(id string) (servicehelp.Doc, error) {
	svc, ok := srvkit.Of(s.ctx, servicehelp.Key)
	if !ok {
		return servicehelp.Doc{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Doc(s.ctx, id)
}
