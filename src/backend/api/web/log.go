// 日志服务: 把"最近记录"以游标方式暴露给前端.
//
// 为什么需要它(见 docs/design/v23.md §47):
//
//	事件(log:stream) 只在监听建立后才送达, 页面没打开/重载期间产生的日志会丢;
//	日志引擎保留最近 N 条, 前端按 seq 游标增量拉取 => 无论是否停在日志页, 记录都不会缺。
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
)

// LogBatch 是一次增量回放结果.
type LogBatch struct {
	Items  []domainlog.Record `json:"items"`
	Seq    uint64             `json:"seq"`
	Oldest uint64             `json:"oldest"`
	Total  int                `json:"total"`
}

// LogService 是绑定到前端的日志服务.
type LogService struct {
	ctx context.Context
}

// NewLogService 创建日志服务绑定.
func NewLogService(ctx context.Context) *LogService {
	return &LogService{ctx: ctx}
}

// Recent 返回 seq 之后的最多 limit 条记录(前端传上一次拿到的 Seq 作为游标).
func (s *LogService) Recent(after uint64, limit int) (LogBatch, error) {
	logger, ok := enginekit.Of[domainlog.Logger](s.ctx, "log")
	if !ok {
		return LogBatch{}, apperr.New(apperr.CodeNotFound)
	}
	reader, ok := logger.(domainlog.RecentReader)
	if !ok {
		return LogBatch{}, apperr.New(apperr.CodeServiceFail)
	}
	items, seq := reader.Recent(after, limit)
	oldest, _, total := reader.RecentInfo()
	if items == nil {
		items = []domainlog.Record{}
	}
	return LogBatch{Items: items, Seq: seq, Oldest: oldest, Total: total}, nil
}
