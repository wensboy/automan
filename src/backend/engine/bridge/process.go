package bridge

import (
	stdctx "context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
	"github.com/wensboy/automan/backend/internal/kits/storekit"
)

// 重启策略取值.
const (
	restartNone   = "none"
	restartOnFail = "on-fail"
	restartAlways = "always"
)

// childProcess 是一次子进程运行的句柄.
type childProcess struct {
	cmd  *exec.Cmd
	done chan struct{} // Wait 返回后关闭
	err  error
}

// materialize 把内嵌产物(**整棵树**)落到运行目录, 并据此设置启动指令。
// 仅在未显式配置命令、且提供了产物时生效; 单文件大小一致则跳过写入(二次启动不会重写)。
// 之所以要整树: dist 里除 index.js 还带 vendor/(whistle 及其依赖), 只落单文件会让
// 打包后的应用找不到 whistle —— 见 docs/design/v19.md §21。
func (e *Engine) materialize(ctx stdctx.Context) error {
	if e.asset == nil || e.command != "" {
		return nil
	}

	root := path.Join("bridge", e.daemon)
	written, skipped := 0, 0
	var total int64
	err := fs.WalkDir(e.asset, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := fs.ReadFile(e.asset, name)
		if readErr != nil {
			return readErr
		}
		rel := path.Join(root, name)
		total += int64(len(data))
		if info, statErr := storekit.Stat(ctx, e.store, rel); statErr == nil && info.Size == int64(len(data)) {
			skipped++
			return nil
		}
		if writeErr := storekit.Write(ctx, e.store, rel, data); writeErr != nil {
			return fmt.Errorf("bridge: materialize %s: %w", rel, writeErr)
		}
		written++
		return nil
	})
	if err != nil {
		return err
	}

	e.command = assetCommand
	e.args = []string{filepath.Join(e.rooter.Root(), filepath.FromSlash(path.Join(root, assetScript)))}
	e.logInfo("bridge: asset ready",
		logkit.F("dir", root), logkit.F("files", written+skipped), logkit.F("written", written), logkit.F("bytes", total))
	return nil
}

// startProcess 启动子进程; 未配置命令时进入附着模式 (子进程由外部启动).
func (e *Engine) startProcess() error {
	if e.command == "" {
		e.logInfo("bridge: attach mode (no command configured)")
		return nil
	}
	cmd := exec.Command(e.command, e.args...)
	cmd.Dir = e.dir
	// 注入监听地址: 子进程据此连入 (argv 优先, env 兜底).
	env := append(os.Environ(), e.env...)
	if server := e.currentServer(); server != nil {
		env = append(env, "AUTOMAN_BRIDGE_SOCK="+server.Addr())
	}
	cmd.Env = env
	cmd.Stdout = e.stdout
	cmd.Stderr = e.stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("bridge: start %q: %w", e.command, err)
	}
	e.setChild(&childProcess{cmd: cmd, done: make(chan struct{})})
	e.logInfo("bridge: child started",
		logkit.F("command", e.command),
		logkit.F("args", e.args),
		logkit.F("pid", cmd.Process.Pid),
		// 只记键名: 值是路径/凭据, 不进日志
		logkit.F("env", envKeys(e.env)))
	return nil
}

// stopProcess 终止子进程: 先 SIGTERM, 宽限期后强杀; 每步都有界, 不会挂住停机 (幂等).
// 注意: 不夺取子进程句柄 —— Wait 由 supervise 协程独占, 夺走会导致进程无法回收
// (且 child.done 永不关闭, 反而把停机拖住).
func (e *Engine) stopProcess() {
	child := e.currentChild()
	if child == nil || child.cmd.Process == nil {
		return
	}
	pid := child.cmd.Process.Pid
	_ = child.cmd.Process.Signal(syscall.SIGTERM)
	if !waitDone(child.done, stopGrace) {
		_ = child.cmd.Process.Kill()
		if !waitDone(child.done, killGrace) {
			e.logError("bridge: child did not exit after kill", logkit.F("pid", pid))
			return
		}
	}
	e.logInfo("bridge: child stopped", logkit.F("pid", pid))
}

// waitDone 有界等待 done 关闭; 返回是否已关闭.
func waitDone(done <-chan struct{}, d time.Duration) bool {
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// waitGroup 有界等待一组协程退出; 返回是否已全部退出.
func waitGroup(wg *sync.WaitGroup, d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	return waitDone(done, d)
}

// supervise 等待子进程退出并按策略重启 (指数退避, 上限 restartMax).
func (e *Engine) supervise(ctx stdctx.Context) {
	defer e.wg.Done()
	attempt := 0
	backoff := backoffBase
	for {
		child := e.currentChild()
		if child == nil {
			return
		}
		pid := child.cmd.Process.Pid
		err := child.cmd.Wait()
		child.err = err
		close(child.done)
		e.setChild(nil)

		// 停机过程中的退出属预期, 不按错误上报.
		if ctx.Err() != nil {
			e.logInfo("bridge: child exited", logkit.F("pid", pid), logkit.F("err", err))
		} else {
			e.logError("bridge: child exited", logkit.F("pid", pid), logkit.F("err", err))
		}
		// 恢复: 重绑监听 (子进程可能已重建 sock), 再按策略重启/等待对端重连.
		if ctx.Err() == nil {
			if rebindErr := e.rebind(); rebindErr != nil {
				e.logError("bridge: rebind failed", logkit.F("err", rebindErr))
			}
		}
		e.emit(event.TopicBridgeDisconnected, event.BridgeDisconnected{
			Daemon: e.daemon,
			Err:    fmt.Sprintf("child exited: %v", err),
		})

		if !e.shouldRestart(ctx, err) || attempt >= e.restartMax {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		attempt++
		if restartErr := e.startProcess(); restartErr != nil {
			e.logError("bridge: restart failed", logkit.F("err", restartErr))
			return
		}
		e.logWarn("bridge: child restarted", logkit.F("attempt", attempt))
		backoff = min(backoff*2, backoffMax)
	}
}

// shouldRestart 按显式策略决定是否重启.
func (e *Engine) shouldRestart(ctx stdctx.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	switch e.restart {
	case restartAlways:
		return true
	case restartOnFail:
		return err != nil
	default:
		return false
	}
}
