// 执行链路适配: 把 bridge 引擎(以领域接口 Sender/Receiver 表达)接成 Starter。
// 服务直接用引擎, 组合根只负责"解析引擎 -> 递进来"(见 docs/design/v19.md §13)。
package proxy

import (
	stdctx "context"
	"errors"
	"fmt"
	"sync"
	"time"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
)

// WithBridge 注入桥接能力: 服务据此创建 starter, 并按消息 id 与 node 回执配表。
// 语义: 发 proxy.start / proxy.stop; node 回 proxy.started / proxy.failed / proxy.stopped。
func WithBridge(sender domainbridge.Sender, receiver domainbridge.Receiver) Option {
	return func(s *Service) {
		starter := &bridgeStarter{
			sender:  sender,
			timeout: maxDuration(s.startWait, time.Second),
			pending: map[string]chan startOutcome{},
		}
		// 引擎的 handlers 表在构造时已初始化, 这里(早于 Start)注册也安全
		receiver.Handle("proxy.started", starter.onStarted)
		receiver.Handle("proxy.failed", starter.onFailed)
		receiver.Handle("proxy.stopped", starter.onStopped)
		s.starter = starter
	}
}

type startOutcome struct {
	info StartInfo
	err  error
}

// bridgeStarter 是 Starter 的 ipc 实现(进程内不依赖具体引擎类型, 便于测试与替换)。
type bridgeStarter struct {
	sender  domainbridge.Sender
	timeout time.Duration

	mu      sync.Mutex
	pending map[string]chan startOutcome
}

// Start 发请求并等待回执; 超时视为失败(是否强杀由 node 侧兜底)。
func (b *bridgeStarter) Start(ctx stdctx.Context, req StartRequest) (StartInfo, error) {
	id := fmt.Sprintf("proxy-start-%d", time.Now().UnixNano())
	ch := b.wait(id)
	defer b.forget(id)

	out := domainbridge.Outgoing{ID: id, Kind: domainbridge.KindStream, Name: "proxy.start", Data: req}
	if err := b.sender.Send(ctx, out); err != nil {
		return StartInfo{}, err
	}
	select {
	case got := <-ch:
		return got.info, got.err
	case <-time.After(b.timeout):
		return StartInfo{}, fmt.Errorf("proxy start timed out after %s", b.timeout)
	}
}

// Stop 发停止请求并等待 proxy.stopped; timeout 为 node 侧的强杀阈值, 这里多等一段再放弃。
func (b *bridgeStarter) Stop(ctx stdctx.Context, proxyID string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	id := fmt.Sprintf("proxy-stop-%d", time.Now().UnixNano())
	ch := b.wait(id)
	defer b.forget(id)

	out := domainbridge.Outgoing{ID: id, Kind: domainbridge.KindStream, Name: "proxy.stop",
		Data: map[string]any{"proxyId": proxyID, "timeoutMs": int(timeout / time.Millisecond)}}
	if err := b.sender.Send(ctx, out); err != nil {
		return err
	}
	// node 侧最多等 timeout 再强杀, 这里留 2s 余量
	select {
	case got := <-ch:
		return got.err
	case <-time.After(timeout + 2*time.Second):
		return fmt.Errorf("proxy stop timed out after %s", timeout+2*time.Second)
	}
}

func (b *bridgeStarter) onStarted(_ stdctx.Context, packet domainbridge.Packet) error {
	var info StartInfo
	if err := packet.Decode(&info); err != nil {
		return err
	}
	b.resolve(packet.ID, startOutcome{info: info})
	return nil
}

func (b *bridgeStarter) onFailed(_ stdctx.Context, packet domainbridge.Packet) error {
	var payload struct {
		ProxyID string `json:"proxyId"`
		Reason  string `json:"reason"`
	}
	if err := packet.Decode(&payload); err != nil {
		return err
	}
	b.resolve(packet.ID, startOutcome{err: errors.New(payload.Reason)})
	return nil
}

func (b *bridgeStarter) onStopped(_ stdctx.Context, packet domainbridge.Packet) error {
	b.resolve(packet.ID, startOutcome{})
	return nil
}

func (b *bridgeStarter) wait(id string) chan startOutcome {
	ch := make(chan startOutcome, 1)
	b.mu.Lock()
	b.pending[id] = ch
	b.mu.Unlock()
	return ch
}

func (b *bridgeStarter) forget(id string) {
	b.mu.Lock()
	delete(b.pending, id)
	b.mu.Unlock()
}

func (b *bridgeStarter) resolve(id string, outcome startOutcome) {
	b.mu.Lock()
	ch, ok := b.pending[id]
	b.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- outcome:
	default:
	}
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
