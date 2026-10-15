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
