// Package event 定义事件总线领域: 引擎间"通知向上"的解耦通道.
package event

// Bus 是事件总线的能力契约.
type Bus interface {
	Emit(topic Topic, payload any)
	Subscribe(topic Topic, fn func(payload any)) (unsubscribe func())
}

// SettingsLoaded 是设置装配完成事件的载荷.
type SettingsLoaded struct {
	Keys int
}

// SettingsChanged 是设置变更事件的载荷.
type SettingsChanged struct {
	Key   string
	Value any
}

// SettingsReloaded 是外部重载事件的载荷; Err 非空表示本次重载失败 (生效值未变).
type SettingsReloaded struct {
	Keys int
	Err  string
}

// SettingsSaved 是落盘完成事件的载荷.
type SettingsSaved struct {
	Keys int
}

// LogDropReason 是日志丢弃/落盘失败的原因.
type LogDropReason string

const (
	LogDropOverflow LogDropReason = "overflow"
	LogDropFlush    LogDropReason = "flush"
)

// LogDropped 是日志丢弃事件的载荷, Count 为本次发生的条数.
type LogDropped struct {
	Reason LogDropReason
	Count  uint64
}

// BridgeConnected 是桥接连接建立事件的载荷.
type BridgeConnected struct {
	Daemon string
	Remote string
}

// BridgeDisconnected 是桥接连接断开事件的载荷.
type BridgeDisconnected struct {
	Daemon string
	Remote string
	Err    string
}

// BridgeError 是桥接出错事件的载荷.
type BridgeError struct {
	Daemon string
	Op     string
	Err    string
}

// SessionSignup 是注册成功事件的载荷.
type SessionSignup struct {
	UserID   string
	Username string
	At       int64
}

// SessionLogin 是登录成功事件的载荷.
type SessionLogin struct {
	UserID   string
	Username string
	At       int64
}

// SessionLogout 是登出事件的载荷.
type SessionLogout struct {
	UserID   string
	Username string
	At       int64
}

// SessionExpired 是会话到期事件的载荷(由 ttlcache 过期钩子发出).
type SessionExpired struct {
	UserID   string
	Username string
	At       int64
}

// NoticeAppended 是一条运行时提示的载荷(前端 toast + 抽屉).
type NoticeAppended struct {
	ID      string
	Label   string
	Title   string
	Content string
	Level   string
	At      int64
}

// NoticeCleared 表示提示队列被清空(所有窗口同步清空).
type NoticeCleared struct {
	At int64
}
