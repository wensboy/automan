// Package context 定义根上下文契约: 只表达运行时骨架 (生命周期 + 引擎表 + 服务表).
// 本包不含任何实现, 实现的唯一归属是 runtime 引擎.
package context

// Mode 是应用的启动形态.
type Mode string

const (
	ModeDesktop Mode = "desktop"
	ModeCLI     Mode = "cli"
)

// Context 是贯穿全链路的根上下文契约.
type Context interface {
	// 生命周期
	Done() <-chan struct{}
	Err() error
	Cancel()
	Mode() Mode

	// ereg: 引擎注册表, 按名称取引擎后再调用其能力
	Engine(name string) (any, bool)

	// srvs: 服务编排表, 只承载编排产物
	Register(key string, srv any) error
	Srv(key string) (any, bool)
}
