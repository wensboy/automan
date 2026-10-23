// Package node 是 bridge 的 **node 运行器**: 负责落盘产物、拉起/收尾子进程并暴露退出信号。
//
// 约定(见 docs/design/v23.md §37):
//   - 启动命令与参数**来自配置**(conf/bridge.json#node.command/args), 运行器不做兜底;
//     命令为空直接报错, 避免"默默用了内置约定"导致排查困难;
//   - 参数里可用 ${AUTOMAN_ASSET} 指代"内嵌产物落盘后的目录"(由运行器替换);
//   - env 的模板展开走 internal/flat(模板 + 输入 -> 输出), 本包只负责"业务规则"(路径不存在即丢弃)。
package node

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/flat"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
	"github.com/wensboy/automan/backend/internal/kits/storekit"
)

const (
	// AssetPlaceholder 在启动参数里指代"产物落盘目录"。
	AssetPlaceholder = "${AUTOMAN_ASSET}"
	// RootPlaceholder 在 env 里指代"仓库根"(仅开发期可解析)。
	RootPlaceholder = "${AUTOMAN_ROOT}"
	stopGrace       = 3 * time.Second
	killGrace       = 3 * time.Second
)

// Params 是 conf/bridge.json#node 中"运行器自己解析"的部分。
//
// Services 等字段只在运行时才有意义, 因此保持 RawMessage: 由运行器按需解析,
// 通用引擎不感知其结构。
type Params struct {
	Command  string          `json:"command"`
	Args     []string        `json:"args"`
	Dir      string          `json:"dir"`
	Env      []string        `json:"env"`
	Services json.RawMessage `json:"services"`
}

// options 是构造参数(与运行态分离; 通过嵌入进入 Runner)。
type options struct {
	asset  fs.FS
	store  domainstore.Store
	rooter domainstore.Rooter
	logger domainlog.Logger
}

// Option 是构造选项。
type Option func(*options)

// WithAsset 注入内嵌产物(用于 ${AUTOMAN_ASSET} 的落盘)。
func WithAsset(asset fs.FS) Option { return func(o *options) { o.asset = asset } }

// WithStore 注入文件存储。
func WithStore(store domainstore.Store) Option { return func(o *options) { o.store = store } }

// WithRooter 注入数据根。
func WithRooter(rooter domainstore.Rooter) Option { return func(o *options) { o.rooter = rooter } }

// WithLogger 注入日志出口(可为 nil)。
func WithLogger(logger domainlog.Logger) Option { return func(o *options) { o.logger = logger } }

type process struct {
	cmd  *exec.Cmd
	done chan struct{}
}

// Runner 是 node 运行器实现。
type Runner struct {
	// options 嵌在首位: 构造参数与自身状态分区明确
	options

	mu    sync.Mutex
	child *process
	info  domainbridge.RunnerInfo
}

// New 创建 node 运行器(不做 IO)。
func New(opts ...Option) *Runner {
	config := options{}
	for _, opt := range opts {
		opt(&config)
	}
	return &Runner{options: config}
}

// Info 实现 domain/bridge.Runner。
func (r *Runner) Info() domainbridge.RunnerInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.info
}

// Start 拉起子进程。
//
// 命令必须由配置提供(不做语言兜底); 参数中的 ${AUTOMAN_ASSET} 会替换为产物落盘目录。
func (r *Runner) Start(ctx stdctx.Context, spec domainbridge.RunnerSpec) error {
	var params Params
	if len(spec.Params) > 0 {
		if err := json.Unmarshal(spec.Params, &params); err != nil {
			return fmt.Errorf("bridge/node: parse params: %w", err)
		}
	}
	command := strings.TrimSpace(params.Command)
	if command == "" {
		return errors.New("bridge/node: command is required (set conf/bridge.json#node.command)")
	}

	args := append([]string(nil), params.Args...)
	if containsAssetPlaceholder(args) {
		dir, err := r.materialize(ctx, spec.Daemon)
		if err != nil {
			return err
		}
		args = replaceAssetPlaceholder(args, dir)
	}

	cmd := exec.Command(command, args...)
	cmd.Dir = params.Dir
	cmd.Env = append(os.Environ(), r.expandEnv(params.Env)...)
	if spec.Sock != "" {
		cmd.Env = append(cmd.Env, "AUTOMAN_BRIDGE_SOCK="+spec.Sock)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("bridge/node: start %q: %w", command, err)
	}

	child := &process{cmd: cmd, done: make(chan struct{})}
	r.mu.Lock()
	r.child = child
	r.info = domainbridge.RunnerInfo{Command: command, Args: args, PID: cmd.Process.Pid}
	r.mu.Unlock()

	r.logInfo("bridge: child started",
		logkit.F("command", command), logkit.F("args", args),
		logkit.F("pid", cmd.Process.Pid), logkit.F("env", flat.KeyNames(params.Env)))
	return nil
}

// Stop 收尾: SIGTERM -> 宽限 -> SIGKILL; 有界且幂等。
func (r *Runner) Stop(stdctx.Context) error {
	r.mu.Lock()
	child := r.child
	r.mu.Unlock()
	if child == nil || child.cmd.Process == nil {
		return nil
	}
	pid := child.cmd.Process.Pid
	_ = child.cmd.Process.Signal(syscall.SIGTERM)
	if waitDone(child.done, stopGrace) {
		r.logInfo("bridge: child stopped", logkit.F("pid", pid))
		return nil
	}
	_ = child.cmd.Process.Kill()
	if waitDone(child.done, killGrace) {
		r.logInfo("bridge: child stopped", logkit.F("pid", pid))
		return nil
	}
	return fmt.Errorf("bridge/node: child %d did not exit after kill", pid)
}

// Wait 等待子进程退出(没有子进程时阻塞到 ctx 结束)。
func (r *Runner) Wait(ctx stdctx.Context) error {
	r.mu.Lock()
	child := r.child
	r.mu.Unlock()
	if child == nil {
		<-ctx.Done()
		return ctx.Err()
	}

	err := child.cmd.Wait()
	close(child.done)

	r.mu.Lock()
	if r.child == child {
		r.child = nil
	}
	r.info = domainbridge.RunnerInfo{}
	r.mu.Unlock()

	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// expandEnv 展开 env 条目(模板替换在 internal/flat, 业务规则在本包):
// 引用了 ${AUTOMAN_ROOT} 但解析不到、或展开后路径不存在的条目会被丢弃。
func (r *Runner) expandEnv(entries []string) []string {
	if len(entries) == 0 {
		return nil
	}
	resolve := func() (string, bool) { return devRoot() }

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		hadPlaceholder := strings.Contains(entry, RootPlaceholder)
		expanded, ok := flat.Resolve(entry, RootPlaceholder, resolve)
		if !ok {
			continue
		}
		if hadPlaceholder && !pathExists(envValue(expanded)) {
			continue
		}
		out = append(out, expanded)
	}
	return out
}

// materialize 把内嵌产物整树落到 <store>/bridge/<daemon>, 返回该目录绝对路径。
func (r *Runner) materialize(ctx stdctx.Context, daemon string) (string, error) {
	if r.asset == nil {
		return "", errors.New("bridge/node: asset is required for " + AssetPlaceholder)
	}
	root := path.Join("bridge", daemon)
	written, skipped, bytes := 0, 0, int64(0)
	err := fs.WalkDir(r.asset, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := fs.ReadFile(r.asset, name)
		if readErr != nil {
			return readErr
		}
		rel := path.Join(root, name)
		bytes += int64(len(data))
		if info, statErr := storekit.Stat(ctx, r.store, rel); statErr == nil && info.Size == int64(len(data)) {
			skipped++
			return nil
		}
		if writeErr := storekit.Write(ctx, r.store, rel, data); writeErr != nil {
			return fmt.Errorf("bridge/node: materialize %s: %w", rel, writeErr)
		}
		written++
		return nil
	})
	if err != nil {
		return "", err
	}
	r.logInfo("bridge: asset ready",
		logkit.F("dir", root), logkit.F("files", written+skipped),
		logkit.F("written", written), logkit.F("bytes", bytes))
	return filepath.Join(r.rooter.Root(), filepath.FromSlash(root)), nil
}

func (r *Runner) logInfo(msg string, fields ...domainlog.Field) {
	if r.logger == nil {
		return
	}
	r.logger.Deliver(logkit.New(domainlog.LevelInfo, msg, fields...))
}

func containsAssetPlaceholder(args []string) bool {
	for _, arg := range args {
		if strings.Contains(arg, AssetPlaceholder) {
			return true
		}
	}
	return false
}

func replaceAssetPlaceholder(args []string, dir string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, strings.ReplaceAll(arg, AssetPlaceholder, dir))
	}
	return out
}

// envValue 取 "KEY=VALUE" 的值部分(无等号时整条视为值)。
func envValue(entry string) string {
	if index := strings.Index(entry, "="); index >= 0 {
		return entry[index+1:]
	}
	return entry
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// devRoot 从工作目录向上找 go.mod(开发期用于解析 ${AUTOMAN_ROOT})。
func devRoot() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		if pathExists(filepath.Join(dir, "go.mod")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func waitDone(done <-chan struct{}, d time.Duration) bool {
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}
