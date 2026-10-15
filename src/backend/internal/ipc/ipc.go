// Package ipc 提供父子进程 IPC 的传输实现: ws over uds.
// 只做传输 (近似 socket): 连接创建与释放、收发 message、心跳与关闭。
// 不解析业务负载, 不按 name 分派, 不维护任何"流"状态。
package ipc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wensboy/automan/backend/domain/bridge"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/fallback"
	"github.com/wensboy/automan/backend/internal/kits/storekit"
)

// 内置缺省.
const (
	defaultMaxMessageBytes = 4 << 20
	defaultUpgradeTimeout  = 5 * time.Second // ws/http 升级超时 (非业务握手)
	defaultSendQueue       = 64
	closeFlushTimeout      = 2 * time.Second
	sockPerm               = 0o600
	probeTimeout           = 300 * time.Millisecond
)

// Option 设置传输参数.
type Option func(*options)

type options struct {
	path            string
	store           domainstore.Store
	rooter          domainstore.Rooter
	maxMessageBytes int64
	upgradeTimeout  time.Duration
	heartbeat       heartbeat
}

// WithPath 设置地址: 相对存储根的 sock 路径 (例 "bridge/node.sock").
func WithPath(path string) Option { return func(o *options) { o.path = path } }

// WithStore 注入 store: 目录创建/探测/删除统一走它.
func WithStore(s domainstore.Store) Option { return func(o *options) { o.store = s } }

// WithRooter 注入存储根: 用于把相对路径解析为绝对路径.
func WithRooter(r domainstore.Rooter) Option { return func(o *options) { o.rooter = r } }

// WithMaxMessageBytes 设置单条消息的字节上限.
func WithMaxMessageBytes(n int64) Option { return func(o *options) { o.maxMessageBytes = n } }

// WithUpgradeTimeout 设置 ws/http 升级超时.
func WithUpgradeTimeout(d time.Duration) Option { return func(o *options) { o.upgradeTimeout = d } }

// WithHeartbeat 设置心跳: 每 interval 发一次 Ping, grace 内未回 Pong 则释放连接
// (interval <= 0 表示关闭心跳).
func WithHeartbeat(interval, grace time.Duration) Option {
	return func(o *options) { o.heartbeat = heartbeat{interval: interval, grace: grace} }
}

func build(opts ...Option) options {
	o := options{
		maxMessageBytes: defaultMaxMessageBytes,
		upgradeTimeout:  defaultUpgradeTimeout,
	}
	for _, opt := range opts {
		opt(&o)
	}
	o.maxMessageBytes = fallback.Int64(o.maxMessageBytes, defaultMaxMessageBytes)
	o.upgradeTimeout = fallback.Duration(o.upgradeTimeout, defaultUpgradeTimeout)
	return o
}

// Server 是服务端的连接创建面.
type Server struct {
	addr  string
	ln    net.Listener
	srv   *http.Server
	peers chan bridge.Peer
	done  chan struct{}
	once  sync.Once
}

// Accept 取一条已完成 ws 升级的连接.
func (s *Server) Accept(ctx context.Context) (bridge.Peer, error) {
	select {
	case p := <-s.peers:
		return p, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, errClosed
	}
}

// Addr 返回监听地址 (sock 绝对路径).
func (s *Server) Addr() string { return s.addr }

// Close 停止接受并关闭监听 (幂等; unix 下顺带 unlink socket 文件).
func (s *Server) Close() error {
	s.once.Do(func() {
		close(s.done)
		_ = s.srv.Close()
		_ = s.ln.Close()
	})
	return nil
}

// Listen 创建被动连接面: 确保目录 -> 处理已存在的 sock -> bind + listen -> 收紧权限.
func Listen(opts ...Option) (*Server, error) {
	o := build(opts...)
	if o.path == "" || o.store == nil || o.rooter == nil {
		return nil, errors.New("ipc: listen requires path, store and rooter")
	}
	addr := filepath.Join(o.rooter.Root(), filepath.FromSlash(o.path))
	ctx := context.Background()
	if err := ensureDir(ctx, o); err != nil {
		return nil, err
	}
	if err := clearStale(ctx, o, addr); err != nil {
		return nil, err
	}
	ln, err := net.Listen("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("ipc: listen %s: %w", addr, err)
	}
	if err := os.Chmod(addr, sockPerm); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("ipc: chmod %s: %w", addr, err)
	}
	return serve(ln, o, addr), nil
}

// Connect 建立主动连接.
func Connect(ctx context.Context, opts ...Option) (bridge.Peer, error) {
	o := build(opts...)
	if o.path == "" || o.rooter == nil {
		return nil, errors.New("ipc: connect requires path and rooter")
	}
	return dial(ctx, filepath.Join(o.rooter.Root(), filepath.FromSlash(o.path)), o)
}

// ensureDir 经 store 建立 sock 所在目录 (IO 统一走 store).
func ensureDir(ctx context.Context, o options) error {
	dir := filepath.ToSlash(filepath.Dir(o.path))
	if dir == "." || dir == "" {
		return nil
	}
	return storekit.Mkdir(ctx, o.store, dir)
}

// clearStale 处理已存在的 sock: 存活实例报错退出, 陈旧 socket 删除后重绑.
func clearStale(ctx context.Context, o options, addr string) error {
	info, err := storekit.Stat(ctx, o.store, o.path)
	if err != nil || info.IsDir {
		return nil // 不存在: 直接绑定
	}
	if probe(addr) {
		return fmt.Errorf("ipc: socket %s is in use by a live instance", addr)
	}
	if !isSocket(addr) {
		return fmt.Errorf("ipc: %s exists and is not a socket", addr)
	}
	if err := storekit.Remove(ctx, o.store, o.path); err != nil {
		return fmt.Errorf("ipc: remove stale socket %s: %w", addr, err)
	}
	return nil
}

// probe 探测路径上是否有可连通的实例.
func probe(addr string) bool {
	conn, err := net.DialTimeout("unix", addr, probeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// isSocket 校验路径确实是 unix socket (避免误删同目录下的普通文件).
func isSocket(addr string) bool {
	fi, err := os.Lstat(addr)
	return err == nil && fi.Mode()&os.ModeSocket != 0
}

var errClosed = errors.New("ipc: closed")
