package bridge

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// 重启策略取值(与 conf/bridge.json 的 restart 对齐)。
const (
	restartNone   = "none"
	restartOnFail = "on-fail"
	restartAlways = "always"
)

// 说明: 引擎是**通用**的 —— 拉起/收尾子进程交给 daemon 对应的 Runner
// (domain/bridge.Runner), 引擎只负责监听/握手/分发与重启编排。

// runnerSpec 组装交给运行器的输入(daemon 专属参数以 RawMessage 透传)。
func (e *Engine) runnerSpec() domainbridge.RunnerSpec {
	return domainbridge.RunnerSpec{
		Daemon: e.daemon,
		Sock:   e.sockPath(),
		Codec:  e.codecName,
		Params: e.params,
	}
}

// startProcess 创建并启动运行器; 未配置 factory 时进入附着模式(子进程由外部启动)。
func (e *Engine) startProcess(ctx stdctx.Context) error {
	if e.factory == nil {
		e.logInfo("bridge: attach mode (no runner configured)")
		return nil
	}
	runner, err := e.factory(domainbridge.RunnerDeps{Store: e.store, Rooter: e.rooter, Logger: e.logger})
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.runner = runner
	e.mu.Unlock()

	// 启动细节(command/pid/env)由运行器自己记录: 通用引擎不需要知道这些
	return runner.Start(ctx, e.runnerSpec())
}

// stopProcess 终止子进程(有界、幂等; 具体信号语义由运行器实现)。
func (e *Engine) stopProcess(ctx stdctx.Context) {
	runner := e.currentRunner()
	if runner == nil {
		return
	}
	if err := runner.Stop(ctx); err != nil {
		e.logError("bridge: child stop failed", logkit.F("err", err))
	}
}

// supervise 等待子进程退出, 重绑监听并按策略重启(指数退避, 上限 restartMax)。
func (e *Engine) supervise(ctx stdctx.Context) {
	defer e.wg.Done()
	attempt := 0
	backoff := backoffBase
	for {
		runner := e.currentRunner()
		if runner == nil {
			return
		}
		err := runner.Wait(ctx)
		if ctx.Err() != nil {
			e.logInfo("bridge: child exited", logkit.F("err", err))
			return
		}
		e.logError("bridge: child exited", logkit.F("err", err))

		// 恢复: 子进程可能已重建 sock, 重绑后再按策略重启/等待对端重连
		if rebindErr := e.rebind(); rebindErr != nil {
			e.logError("bridge: rebind failed", logkit.F("err", rebindErr))
		}
		e.emitBridgeDisconnected(err)

		if !shouldRestart(e.restart, err) || attempt >= e.restartMax {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		attempt++
		if restartErr := e.startProcess(ctx); restartErr != nil {
			e.logError("bridge: restart failed", logkit.F("err", restartErr))
			return
		}
		e.logWarn("bridge: child restarted", logkit.F("attempt", attempt))
		backoff = min(backoff*2, backoffMax)
	}
}

func shouldRestart(policy string, err error) bool {
	switch policy {
	case restartAlways:
		return true
	case restartOnFail:
		return err != nil
	default:
		return false
	}
}

// emitBridgeDisconnected 广播"子进程退出"(前端/服务据此更新状态)。
func (e *Engine) emitBridgeDisconnected(err error) {
	e.emit(event.TopicBridgeDisconnected, event.BridgeDisconnected{
		Daemon: e.daemon,
		Err:    fmt.Sprintf("child exited: %v", err),
	})
}

// waitGroup 有界等待一组协程退出; 返回是否已全部退出。
func waitGroup(wg *sync.WaitGroup, d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// marshalParams 把 daemon 配置转成 RawMessage(运行器自行解析 services 等字段)。
func marshalParams(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}
