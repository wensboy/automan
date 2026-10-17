package event

// Topic 是事件主题.
type Topic string

const (
	// TopicSettingsLoaded 在设置装配完成时广播.
	TopicSettingsLoaded Topic = "settings.loaded"
	// TopicSettingsChanged 在运行态设置变更时广播.
	TopicSettingsChanged Topic = "settings.changed"
	// TopicSettingsReloaded 在外部编辑触发的重载后广播.
	TopicSettingsReloaded Topic = "settings.reloaded"
	// TopicSettingsSaved 在叠加回写用户文件完成后广播.
	TopicSettingsSaved Topic = "settings.saved"
	// TopicLogSync 请求把内存缓冲的日志落盘.
	TopicLogSync Topic = "log.sync"
	// TopicLogDropped 在日志溢出或落盘失败时广播.
	TopicLogDropped Topic = "log.dropped"
	// TopicSessionSignup 在用户注册成功时广播.
	TopicSessionSignup Topic = "session.signup"
	// TopicSessionLogin 在登录成功时广播.
	TopicSessionLogin Topic = "session.login"
	// TopicSessionLogout 在登出时广播.
	TopicSessionLogout Topic = "session.logout"
	// TopicSessionExpired 在会话到期(ttlcache 过期钩子)时广播.
	TopicSessionExpired Topic = "session.expired"

	// TopicBridgeConnected 在桥接连接建立时广播.
	TopicBridgeConnected Topic = "bridge.connected"
	// TopicBridgeDisconnected 在桥接连接断开时广播.
	TopicBridgeDisconnected Topic = "bridge.disconnected"
	// TopicBridgeError 在桥接出错时广播.
	TopicBridgeError Topic = "bridge.error"
)
