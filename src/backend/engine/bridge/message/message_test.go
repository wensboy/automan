package message

import (
	stdctx "context"
	"path/filepath"
	"testing"
	"time"

	"github.com/wensboy/automan/backend/engine/bridge/frame"
	"github.com/wensboy/automan/backend/engine/bridge/transport"
)

// 起一对真实 UDS + WS 连接(服务端/客户端), 返回两侧 peer。
func newPair(t *testing.T, serverOptions, clientOptions Options) (*Peer, *Peer) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bridge.sock")
	listener, err := transport.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	serverCh := make(chan *Peer, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept(stdctx.Background())
		if acceptErr != nil {
			errCh <- acceptErr
			return
		}
		framed, upgradeErr := frame.ServerUpgrade(stdctx.Background(), conn, 3*time.Second, frame.Options{})
		if upgradeErr != nil {
			errCh <- upgradeErr
			return
		}
		serverCh <- New(framed, serverOptions)
	}()

	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
	defer cancel()
	raw, err := transport.Dial(ctx, path, 3*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	framed, err := frame.ClientUpgrade(ctx, raw, 3*time.Second, frame.Options{})
	if err != nil {
		t.Fatalf("ClientUpgrade: %v", err)
	}
	client := New(framed, clientOptions)

	select {
	case server := <-serverCh:
		t.Cleanup(func() {
			server.Close(stdctx.Background(), "test done")
			client.Close(stdctx.Background(), "test done")
		})
		return server, client
	case err := <-errCh:
		t.Fatalf("server: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("等待服务端 peer 超时")
	}
	return nil, nil
}

// 请求-响应: 关联 ID、数据往返、错误帧映射。
func TestRequestResponseAndRemoteError(t *testing.T) {
	server, client := newPair(t, Options{}, Options{})
	server.Handle("echo", func(_ stdctx.Context, env Envelope) (any, error) {
		return map[string]any{"got": string(env.Data)}, nil
	})
	server.Handle("boom", func(_ stdctx.Context, _ Envelope) (any, error) {
		return nil, stdctx.DeadlineExceeded
	})

	data, err := client.Request(stdctx.Background(), "echo", map[string]string{"k": "v"}, 2*time.Second)
	if err != nil {
		t.Fatalf("Request(echo): %v", err)
	}
	if string(data) == "" || string(data) != `{"got":"{\"k\":\"v\"}"}` {
		t.Fatalf("响应数据异常: %s", data)
	}

	if _, err := client.Request(stdctx.Background(), "boom", nil, 2*time.Second); err == nil {
		t.Fatal("处理失败应返回 remote error")
	} else if _, ok := err.(*RemoteError); !ok {
		t.Fatalf("应为 RemoteError, got %T %v", err, err)
	}

	// 未注册的请求: 明确错误(而不是超时)
	if _, err := client.Request(stdctx.Background(), "missing", nil, 2*time.Second); err == nil {
		t.Fatal("未注册 handler 应报错")
	}
}

// 事件: 单向通知, 对端按 name 分发。
func TestNotifyEvent(t *testing.T) {
	server, client := newPair(t, Options{}, Options{})
	received := make(chan string, 1)
	client.OnEvent("tick", func(env Envelope) {
		received <- string(env.Data)
	})

	if err := server.Notify(stdctx.Background(), "tick", map[string]int{"n": 1}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	select {
	case got := <-received:
		if got != `{"n":1}` {
			t.Fatalf("事件数据异常: %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未收到事件")
	}
}

// 超时: 对端不回响应 -> ErrTimeout, 且 pending 被清理。
func TestRequestTimeout(t *testing.T) {
	server, client := newPair(t, Options{}, Options{})
	server.Handle("slow", func(ctx stdctx.Context, _ Envelope) (any, error) {
		time.Sleep(300 * time.Millisecond)
		return "late", nil
	})

	if _, err := client.Request(stdctx.Background(), "slow", nil, 50*time.Millisecond); err != ErrTimeout {
		t.Fatalf("应为 ErrTimeout, got %v", err)
	}
	client.mu.Lock()
	left := len(client.pending)
	client.mu.Unlock()
	if left != 0 {
		t.Fatalf("超时后 pending 应清空, got %d", left)
	}
}

// 背压: 发送队列满 -> 立即 ErrBusy(不阻塞调用方)。
// 直接构造 peer 的队列面(不启动写协程), 避免"写协程恰好消费掉一条"的时序抖动。
func TestEnqueueBusy(t *testing.T) {
	peer := &Peer{
		options: Options{},
		writeCh: make(chan Envelope, 1),
		done:    make(chan struct{}),
	}
	peer.writeCh <- Envelope{Kind: KindEvent, Name: "fill"}

	if err := peer.enqueue(stdctx.Background(), Envelope{Kind: KindEvent, Name: "x"}, nil); err != ErrBusy {
		t.Fatalf("队列满应返回 ErrBusy, got %v", err)
	}
}
