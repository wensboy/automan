package bridge

import (
	stdctx "context"
	"encoding/json"

	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
)

// RunnerSpec 是引擎交给 runner 的全部输入。
//
// 设计(docs/design/v23.md §36):
//   - 引擎是**通用**的: 只负责监听/握手/消息分发/连接生命周期, 不知道子进程是什么语言;
//   - 拉起与收尾由 daemon 对应的 Runner 负责;
//   - daemon 专属参数以 **json.RawMessage** 传入(结构参考 conf/bridge.json#<daemon>),
//     由 runner 自行解析 —— 例如 services 这类"运行时才能确认"的字段归 runner 管。
type RunnerSpec struct {
	// Daemon 是 daemon 名(缺省 "" 表示不拉起子进程/附着模式)。
	Daemon string
	// Sock 是**已解析**的监听地址(引擎已完成相对路径到绝对路径的转换)。
	Sock string
	// Codec 是业务负载编码名(握手用)。
	Codec string
	// Params 是该 daemon 的原始配置(conf/bridge.json#<daemon> 的 JSON)。
	Params json.RawMessage
}

// RunnerInfo 是运行器的可观测信息(仅用于日志/诊断)。
type RunnerInfo struct {
	Command string
	Args    []string
	PID     int
}

// Runner 是"子进程运行器"契约。
type Runner interface {
	// Start 启动子进程; 附着模式下应直接返回 nil。
	Start(ctx stdctx.Context, spec RunnerSpec) error
	// Stop 收尾(有界、幂等)。
	Stop(ctx stdctx.Context) error
	// Wait 阻塞到子进程退出; 没有子进程时阻塞到 ctx 结束。
	Wait(ctx stdctx.Context) error
	// Info 返回当前进程信息(未启动时字段为空)。
	Info() RunnerInfo
}

// RunnerDeps 是引擎交给工厂的公共依赖(所有 daemon 都可能用到)。
type RunnerDeps struct {
	Store  domainstore.Store
	Rooter domainstore.Rooter
	Logger domainlog.Logger
}

// RunnerFactory 由具体 daemon 提供; 引擎按 daemon 名取用。
//
// 返回 nil 表示该 daemon 不接管子进程(引擎进入附着模式)。
type RunnerFactory func(deps RunnerDeps) (Runner, error)
