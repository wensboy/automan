// 执行链路适配: 把 bridge 引擎(以领域接口 Sender/Receiver 表达)接成 Starter。
// 服务直接用引擎, 组合根只负责"解析引擎 -> 递进来"(见 docs/design/v19.md §13)。
package proxy

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"time"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
)

// WithBridge 注入桥接能力: 服务据此创建 starter, 并按消息 id 与 node 回执配表。
// 语义: 发 proxy.start / proxy.stop; node 回 proxy.started / proxy.failed / proxy.stopped。
func (s *Service) WithBridge(requester domainbridge.Requester) *Service {
	s.starter = &bridgeStarter{
		requester: requester,
		timeout:   startTimeout(s.startWait) + 5*time.Second,
	}
	return s
}

type startOutcome struct {
	info StartInfo
	err  error
}

// bridgeStarter 是 Starter 的桥接实现: 用**请求-响应**语义(见 docs/design/v23.md §3.3)。
//
// 说明(v23 §18 的教训): 早先用"业务自己按消息 ID 配表"的写法, 而 node 回执走事件通道,
// ID 在事件里不被保留 -> 批量启动时逐一落空直到超时。改为 Requester 后由消息层关联。
type bridgeStarter struct {
	requester domainbridge.Requester
	timeout   time.Duration
}

// Start 发请求并等待回执(消息层关联 ID, 超时视为失败)。
func (b *bridgeStarter) Start(ctx stdctx.Context, req StartRequest) (StartInfo, error) {
	raw, err := b.requester.Request(ctx, "proxy.start", req, b.timeout)
	if err != nil {
		return StartInfo{}, err
	}
	var info StartInfo
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &info); err != nil {
			return StartInfo{}, fmt.Errorf("proxy start: decode reply: %w", err)
		}
	}
	return info, nil
}

// Stop 发停止请求并等待回执; timeout 为 node 侧的强杀阈值, 这里多等一段再放弃。
func (b *bridgeStarter) Stop(ctx stdctx.Context, proxyID, dir string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	_, err := b.requester.Request(ctx, "proxy.stop",
		map[string]any{"proxyId": proxyID, "dir": dir, "timeoutMs": int(timeout / time.Millisecond)},
		timeout+5*time.Second)
	return err
}

// startTimeout 给启动等待一个下限(startWait 未配置时也要有边界)。
func startTimeout(wait time.Duration) time.Duration {
	if wait < time.Second {
		return 15 * time.Second
	}
	return wait
}
