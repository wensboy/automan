package log

import "time"

// Notice 是一条**运行时提示**(面向人, 不持久化): 与日志同源, 但只保留最近若干条.
type Notice struct {
	ID      string
	Time    time.Time
	Level   Level
	Label   string // 分类标签(例 proxy / session), 用于抽屉分组与搜索
	Title   string // 一句话结论
	Content string // 细节(可空)
}

// Notifier 是运行时提示的投递契约.
//
// 与 Logger 刻意分开(不复用):
//   - Logger 面向"完整记录", 目标是 disk/console/stream;
//   - Notifier 面向"人", 目标是 notify out(内存环形) + 事件转发到 UI, 且可被清除.
type Notifier interface {
	// Notify 投递一条提示(非阻塞; ID 为空时由实现生成).
	Notify(notice Notice)
	// Notices 返回当前队列(最新在前), 供 UI 首次拉取.
	Notices() []Notice
	// ClearNotices 清空队列(UI 点"清除"), 并广播 cleared 事件.
	ClearNotices()
}
