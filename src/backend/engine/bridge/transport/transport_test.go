package transport

import (
	stdctx "context"
	"io"
	"path/filepath"
	"testing"
	"time"
)

// 监听/接线/部分读写 + 关闭时 unlink。
func TestListenDialReadWriteAndUnlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.sock")
	listener, err := Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	accepted := make(chan Conn, 1)
	go func() {
		conn, acceptErr := listener.Accept(stdctx.Background())
		if acceptErr == nil {
			accepted <- conn
		}
	}()

	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
	defer cancel()
	client, err := Dial(ctx, path, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	var server Conn
	select {
	case server = <-accepted:
	case <-time.After(3 * time.Second):
		t.Fatal("Accept 超时")
	}
	defer server.Close()

	// 大报文: 触发多次内核写/读(部分读写路径)
	payload := make([]byte, 256*1024)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	go func() { _ = client.Write(ctx, payload) }()

	got := make([]byte, len(payload))
	if _, err := io.ReadFull(server, got); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatal("内容不一致")
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := Dial(ctx, path, 300*time.Millisecond); err == nil {
		t.Fatal("关闭后不应再能连接")
	}
}

// socket 已存在且有人在听 -> 拒绝抢占; 关闭后残留文件可被重新监听。
func TestListenRejectsLiveAndRecoversStale(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.sock")
	first, err := Listen(path)
	if err != nil {
		t.Fatalf("Listen(first): %v", err)
	}
	if _, err := Listen(path); err == nil {
		t.Fatal("有人在听时应拒绝重复监听")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := Listen(path)
	if err != nil {
		t.Fatalf("关闭后应可重新监听: %v", err)
	}
	_ = second.Close()
}

// 读超时: deadline 到期返回 ErrTimeout。
func TestReadTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timeout.sock")
	listener, err := Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, acceptErr := listener.Accept(stdctx.Background())
		if acceptErr == nil {
			// 保持连接一小会儿: 让对端先撞到读超时, 而不是立刻收到 EOF
			time.Sleep(600 * time.Millisecond)
			_ = conn.Close()
		}
	}()

	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 2*time.Second)
	defer cancel()
	client, err := Dial(ctx, path, time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	readCtx, readCancel := stdctx.WithTimeout(stdctx.Background(), 100*time.Millisecond)
	defer readCancel()
	buf := make([]byte, 16)
	if _, err := client.ReadContext(readCtx, buf); err != ErrTimeout {
		t.Fatalf("应为 ErrTimeout, got %v", err)
	}
}
