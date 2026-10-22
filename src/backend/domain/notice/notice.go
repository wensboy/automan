// Package notice 定义"运行时提示(notification)"的领域契约.
//
// 约定(docs/design/v23.md §32):
//   - **业务操作结束后由服务层 Push**; 前端只负责展示(事件驱动), 不再各自 toast;
//   - 前端侧处理(本地校验等)也可以用同一入口, 保证"弹过的都在抽屉里能回看";
//   - 勿扰、去重、容量等策略由实现(service/notice)负责, 服务层不感知。
package notice

// Level 是提示级别(与前端 toast 语义对齐)。
type Level string

const (
	// LevelInfo 是普通信息。
	LevelInfo Level = "info"
	// LevelSuccess 是成功结果。
	LevelSuccess Level = "success"
	// LevelWarn 是需要留意的结果。
	LevelWarn Level = "warn"
	// LevelError 是失败结果。
	LevelError Level = "error"
)

// Entry 是一条提示(id/at 由实现生成; 投递时留空)。
type Entry struct {
	ID      string `json:"id"`
	Level   string `json:"level"`
	Label   string `json:"label"`
	Title   string `json:"title"`
	Content string `json:"content"`
	At      int64  `json:"at"`
}

// Publisher 是投递能力(实现见 service/notice)。
type Publisher interface {
	// Push 投递一条提示; 返回落库形态(id 为空表示被勿扰抑制)。
	Push(entry Entry) (Entry, error)
}
