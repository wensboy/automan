package bridge_test

import (
	stdctx "context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
	"github.com/wensboy/automan/backend/engine/bridge/frame"
	"github.com/wensboy/automan/backend/engine/bridge/message"
	"github.com/wensboy/automan/backend/engine/bridge/transport"
)

// e2e: 起**真实的 node 子进程**(dist/index.js), 验证 v3 线协议两侧一致:
// 握手 -> node.ready 事件 -> node.echo 请求/响应 -> node.log 事件。
func TestE2EWithRealNodeProcess(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node 不在 PATH, 跳过 e2e")
	}
	script, err := filepath.Abs("../../bridge/node/dist/index.js")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	socket := filepath.Join(t.TempDir(), "e2e.sock")
	listener, err := transport.Listen(socket)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer listener.Close()

	peers := make(chan *message.Peer, 1)
	errs := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept(stdctx.Background())
		if acceptErr != nil {
			errs <- acceptErr
			return
		}
		framed, upgradeErr := frame.ServerUpgrade(stdctx.Background(), conn, 5*time.Second, frame.Options{})
		if upgradeErr != nil {
			errs <- upgradeErr
			return
		}
		peers <- message.New(framed, message.Options{
			RequestTimeout: 5 * time.Second,
			Prepare: func(p *message.Peer) {
				p.Handle("bridge.handshake", func(_ stdctx.Context, env message.Envelope) (any, error) {
					var request domainbridge.Handshake
					if err := json.Unmarshal(env.Data, &request); err != nil {
						return nil, err
					}
					if request.Version != message.Version {
						return nil, stdctx.DeadlineExceeded
					}
					return domainbridge.HandshakeResult{Daemon: "node", Codec: "json", Version: message.Version}, nil
				})
			},
		})
	}()

	cmd := exec.Command("node", script, "--sock="+socket, "--daemon=node", "--codec=json")
	cmd.Env = append(cmd.Environ(), "BRIDGE_DEBUG=1")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start node: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	var peer *message.Peer
	select {
	case peer = <-peers:
	case err := <-errs:
		t.Fatalf("server: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("等待子进程连接超时")
	}

	ready := make(chan struct{}, 1)
	logs := make(chan string, 1)
	peer.OnEvent("node.ready", func(message.Envelope) { ready <- struct{}{} })
	peer.OnEvent("node.log", func(env message.Envelope) {
		var record domainbridge.NodeLog
		if err := json.Unmarshal(env.Data, &record); err == nil {
			logs <- record.Msg
		}
	})

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("未收到 node.ready")
	}
	select {
	case msg := <-logs:
		if msg == "" {
			t.Fatal("node.log 消息为空")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未收到 node.log")
	}

	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
	defer cancel()
	data, err := peer.Request(ctx, "node.echo", map[string]string{"hello": "world"}, 3*time.Second)
	if err != nil {
		t.Fatalf("node.echo: %v", err)
	}
	var echoed map[string]string
	if err := json.Unmarshal(data, &echoed); err != nil {
		t.Fatalf("解析响应: %v raw=%s", err, data)
	}
	if echoed["hello"] != "world" {
		t.Fatalf("回显不一致: %+v", echoed)
	}

	// 优雅关闭: 发 close 帧, 子进程应正常退出(若卡住会被 Cleanup 强杀)
	peer.Close(stdctx.Background(), "e2e done")
}
