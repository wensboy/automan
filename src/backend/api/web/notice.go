// 提示服务: 把运行时提示(notification)的队列与投递暴露给前端.
//
// 数据源是独立服务 service/notice(内存环形, 不持久化); 前端 toast 与抽屉都走同一条链路:
// 先投递(前端或后端) -> 服务广播事件 -> 前端追加并 toast。见 docs/design/v23.md §28。
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	servicenotice "github.com/wensboy/automan/backend/service/notice"
)

// NoticeService 是绑定到前端的提示服务.
type NoticeService struct {
	ctx context.Context
}

// NewNoticeService 创建提示服务绑定.
func NewNoticeService(ctx context.Context) *NoticeService {
	return &NoticeService{ctx: ctx}
}

// List 返回当前队列(最新在前).
func (s *NoticeService) List() ([]servicenotice.Entry, error) {
	svc, ok := srvkit.Of(s.ctx, servicenotice.Key)
	if !ok {
		return []servicenotice.Entry{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.List(s.ctx), nil
}

// Push 投递一条提示(前端触发的业务提示也走这里, 保证与后端触发同源).
func (s *NoticeService) Push(level, label, title, content string) (servicenotice.Entry, error) {
	svc, ok := srvkit.Of(s.ctx, servicenotice.Key)
	if !ok {
		return servicenotice.Entry{}, apperr.New(apperr.CodeNotFound)
	}
	return svc.Push(servicenotice.Entry{Level: level, Label: label, Title: title, Content: content})
}

// Clear 清空后端队列(UI 点"清除").
func (s *NoticeService) Clear() error {
	svc, ok := srvkit.Of(s.ctx, servicenotice.Key)
	if !ok {
		return apperr.New(apperr.CodeNotFound)
	}
	svc.Clear(s.ctx)
	return nil
}
