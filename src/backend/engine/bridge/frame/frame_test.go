package frame

import (
	stdctx "context"
	"path/filepath"
	"testing"
	"time"

	"github.com/wensboy/automan/backend/engine/bridge/transport"
)

func pair(t *testing.T) (*Conn, *Conn) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f.sock")
	listener, err := transport.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	serverCh := make(chan *Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept(stdctx.Background())
		if acceptErr != nil {
			errCh <- acceptErr
			return
		}
		framed, upgradeErr := ServerUpgrade(stdctx.Background(), conn, 3*time.Second, Options{})
		if upgradeErr != nil {
			errCh <- upgradeErr
			return
		}
		serverCh <- framed
	}()

	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
	defer cancel()
	raw, err := transport.Dial(ctx, path, 3*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	client, err := ClientUpgrade(ctx, raw, 3*time.Second, Options{})
	if err != nil {
		t.Fatalf("ClientUpgrade: %v", err)
	}
	select {
	case server := <-serverCh:
		t.Cleanup(func() { _ = server.Close(stdctx.Background(), CloseNormal, "") })
		return server, client
	case err := <-errCh:
		t.Fatalf("server: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("等待服务端 upgrade 超时")
	}
	return nil, nil
}

// 握手 + 双向文本消息 + 长度边界(125/126/65536)。
func TestUpgradeAndMessageLengths(t *testing.T) {
	server, client := pair(t)
	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
	defer cancel()

	sizes := []int{5, 125, 126, 65535, 65536}
	for _, size := range sizes {
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = byte('a' + i%26)
		}
		if err := client.WriteMessage(ctx, OpText, payload); err != nil {
			t.Fatalf("write %d: %v", size, err)
		}
		op, got, err := server.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("read %d: %v", size, err)
		}
		if op != OpText || len(got) != size {
			t.Fatalf("长度 %d 异常: op=%v len=%d", size, op, len(got))
		}

		if err := server.WriteMessage(ctx, OpText, got); err != nil {
			t.Fatalf("echo %d: %v", size, err)
		}
		if _, echoed, err := client.ReadMessage(ctx); err != nil || len(echoed) != size {
			t.Fatalf("echo 读回 %d 失败: err=%v len=%d", size, err, len(echoed))
		}
	}
}

// 分片重组: text(FIN=0) + continuation(FIN=0) + continuation(FIN=1) 应得到一条完整消息。
func TestFragmentation(t *testing.T) {
	server, client := pair(t)
	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
	defer cancel()

	writeRaw := func(fin bool, op Opcode, payload string) {
		head := []byte{byte(op)}
		if fin {
			head[0] |= 0x80
		}
		head = append(head, byte(len(payload)))
		if err := client.raw.Write(ctx, append(head, []byte(payload)...)); err != nil {
			t.Fatalf("raw write: %v", err)
		}
	}
	writeRaw(false, OpText, "he")
	writeRaw(false, OpContinuation, "ll")
	writeRaw(true, OpContinuation, "o")

	op, payload, err := server.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if op != OpText || string(payload) != "hello" {
		t.Fatalf("分片重组结果异常: op=%v payload=%q", op, payload)
	}
}
