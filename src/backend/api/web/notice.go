// 提示服务: 把 log 引擎里的运行时提示(内存队列)暴露给前端 notification 抽屉.
// 说明: 提示不持久化; 清除后即从后端队列消失(见 docs/design/v20.md)。
package web

import (
	"time"

	"github.com/wensboy/automan/backend/domain/context"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
)

// Notice 是对前端的提示形态(时间用毫秒, 避免绑定层处理 time.Time).
type Notice struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Level   string `json:"level"`
	At      int64  `json:"at"`
}

// NoticeService 是绑定到前端的提示服务.
type NoticeService struct {
	ctx context.Context
}

// NewNoticeService 创建提示服务绑定.
func NewNoticeService(ctx context.Context) *NoticeService {
	return &NoticeService{ctx: ctx}
}

// List 返回当前队列(最新在前).
func (s *NoticeService) List() []Notice {
	notifier, ok := enginekit.Of[domainlog.Notifier](s.ctx, "log")
	if !ok {
		return []Notice{}
	}
	items := notifier.Notices()
	out := make([]Notice, 0, len(items))
	for _, item := range items {
		out = append(out, Notice{
			ID: item.ID, Label: item.Label, Title: item.Title, Content: item.Content,
			Level: item.Level.String(), At: item.Time.UnixMilli(),
		})
	}
	return out
}

// Clear 清空后端队列(UI 点"清除").
func (s *NoticeService) Clear() {
	notifier, ok := enginekit.Of[domainlog.Notifier](s.ctx, "log")
	if !ok {
		return
	}
	notifier.ClearNotices()
}

// ensure time import used if Notice.At changes
var _ = time.Now
