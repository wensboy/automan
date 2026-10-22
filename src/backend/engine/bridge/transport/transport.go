// Package transport 管理 UDS 字节流: 监听/接线/部分读写/deadline/关闭。
//
// 约束(见 docs/design/v23.md §3.1): 只认 []byte 与 deadline, 不认识 WebSocket, 也不认识 JSON。
package transport

import (
	stdctx "context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
)

// 错误与接口复用 domain/bridge 的通用契约(实现细节不外泄)。
var (
	ErrClosed       = domainbridge.ErrClosed
	ErrTimeout      = domainbridge.ErrTimeout
	ErrBackpressure = domainbridge.ErrBackpressure
)

// Conn 是一条已建立的字节流连接。
//
// Read/Write 都尊重 ctx: ctx 到期立即返回 ErrTimeout(内部用 deadline 实现, 不改动调用方的 ctx)。
// Conn 是 domain/bridge.Stream 的具体实现形态。
type Conn = domainbridge.Stream

// 编译期断言: 监听器与连接都满足领域契约。
var _ domainbridge.Acceptor = (*Listener)(nil)

// Listener 是 UDS 监听器。
type Listener struct {
	inner net.Listener
	path  string
	// owned 表示 socket 文件由本进程创建: 关闭时需要 unlink(避免误删别人的 socket)。
	owned bool
}

// Listen 在 path 上监听 UDS。
//
// 启动策略(v23 §8): 若文件已存在, 先探测是否有人在听 —— 有人在听直接报错(不抢占);
// 无人监听(残留文件)则删掉重建。
func Listen(path string) (*Listener, error) {
	if path == "" {
		return nil, errors.New("bridge/transport: socket path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("bridge/transport: create socket dir: %w", err)
	}
	owned := true
	if info, err := os.Stat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("bridge/transport: %s exists and is not a socket", path)
		}
		if alive(path) {
			return nil, fmt.Errorf("bridge/transport: socket %s is in use by a live instance", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("bridge/transport: remove stale socket: %w", err)
		}
	}
	inner, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("bridge/transport: listen %s: %w", path, err)
	}
	// 仅本用户可访问
	_ = os.Chmod(path, 0o600)
	return &Listener{inner: inner, path: path, owned: owned}, nil
}

// Dial 连接一个 UDS 端点(Go 侧客户端/测试用)。
func Dial(ctx stdctx.Context, path string, timeout time.Duration) (Conn, error) {
	dialer := net.Dialer{Timeout: timeout}
	raw, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, err
	}
	return &conn{inner: raw}, nil
}

// Accept 等待一条新连接。
func (l *Listener) Accept(ctx stdctx.Context) (Conn, error) {
	type accepted struct {
		conn net.Conn
		err  error
	}
	done := make(chan accepted, 1)
	go func() {
		conn, err := l.inner.Accept()
		done <- accepted{conn: conn, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case result := <-done:
		if result.err != nil {
			if errors.Is(result.err, net.ErrClosed) {
				return nil, ErrClosed
			}
			return nil, result.err
		}
		return &conn{inner: result.conn}, nil
	}
}

// Addr 返回 socket 路径。
func (l *Listener) Addr() string { return l.path }

// Close 停止监听并按需 unlink socket 文件。
func (l *Listener) Close() error {
	err := l.inner.Close()
	if l.owned {
		_ = os.Remove(l.path)
	}
	if err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

type conn struct {
	inner net.Conn
}

func (c *conn) Read(p []byte) (int, error) {
	n, err := c.inner.Read(p)
	if err != nil {
		return n, normalize(err)
	}
	return n, nil
}

func (c *conn) ReadContext(ctx stdctx.Context, p []byte) (int, error) {
	if err := c.applyDeadline(ctx, true); err != nil {
		return 0, err
	}
	return c.Read(p)
}

func (c *conn) Write(ctx stdctx.Context, p []byte) error {
	if err := c.applyDeadline(ctx, false); err != nil {
		return err
	}
	written := 0
	for written < len(p) {
		n, err := c.inner.Write(p[written:])
		if err != nil {
			if isTimeout(err) {
				return ErrBackpressure
			}
			return normalize(err)
		}
		written += n
	}
	return nil
}

func (c *conn) SetReadDeadline(t time.Time) error  { return c.inner.SetReadDeadline(t) }
func (c *conn) SetWriteDeadline(t time.Time) error { return c.inner.SetWriteDeadline(t) }
func (c *conn) Close() error                       { return c.inner.Close() }
func (c *conn) Local() string                      { return c.inner.LocalAddr().String() }
func (c *conn) Remote() string                     { return c.inner.RemoteAddr().String() }

// applyDeadline 把 ctx 的到期时间同步到 socket deadline(避免依赖 goroutine 轮询)。
func (c *conn) applyDeadline(ctx stdctx.Context, read bool) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Time{}
	}
	var err error
	if read {
		err = c.inner.SetReadDeadline(deadline)
	} else {
		err = c.inner.SetWriteDeadline(deadline)
	}
	if err != nil {
		return normalize(err)
	}
	if err := ctx.Err(); err != nil {
		return ErrTimeout
	}
	return nil
}

func normalize(err error) error {
	var netErr net.Error
	switch {
	case errors.Is(err, net.ErrClosed), errors.Is(err, os.ErrClosed):
		return ErrClosed
	case errors.As(err, &netErr) && netErr.Timeout():
		return ErrTimeout
	default:
		return err
	}
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// alive 探测 socket 上是否有进程在监听。
func alive(path string) bool {
	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
