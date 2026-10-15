package ipc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/wensboy/automan/backend/domain/bridge"
)

// heartbeat 是心跳参数: 每 interval 发一次 Ping, grace 内未回 Pong 判定失联.
type heartbeat struct {
	interval time.Duration
	grace    time.Duration
}

// peer 是一条 ipc 连接 (近似 socket): 阻塞读 + 单写泵 + 可选心跳.
// 构造时不启动任何循环; 连接就绪后由 start 启动.
type peer struct {
	conn   *websocket.Conn
	opts   options
	local  string
	remote string

	out    chan []byte
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{} // 写泵退出后关闭
	drain  chan struct{} // 请求写泵冲刷后退出
	once   sync.Once     // 保证 Close 幂等 (drain 只关一次)
}

func newPeer(conn *websocket.Conn, o options, local, remote string) *peer {
	return &peer{
		conn:   conn,
		opts:   o,
		local:  local,
		remote: remote,
		out:    make(chan []byte, defaultSendQueue),
		done:   make(chan struct{}),
		drain:  make(chan struct{}),
	}
}

// start 启动内部循环 (写泵 + 可选心跳); 由连接建立路径调用, 构造期不调用.
func (p *peer) start() {
	p.ctx, p.cancel = context.WithCancel(context.Background())
	go p.pump()
	if p.opts.heartbeat.interval > 0 {
		go p.beat()
	}
}

// pump 是唯一写者: 顺序消费发送队列 (写串行 + 顺序保持).
func (p *peer) pump() {
	defer close(p.done)
	for {
		select {
		case data := <-p.out:
			if err := p.conn.Write(p.ctx, websocket.MessageText, data); err != nil {
				p.conn.CloseNow()
				return
			}
		case <-p.drain:
			p.flush()
			return
		case <-p.ctx.Done():
			return
		}
	}
}

// flush 尽力写完已入队的帧: 写入受有界 ctx 约束, 卡在网络上也能退出.
func (p *peer) flush() {
	ctx, cancel := context.WithTimeout(p.ctx, closeFlushTimeout)
	defer cancel()
	for {
		select {
		case data := <-p.out:
			if err := p.conn.Write(ctx, websocket.MessageText, data); err != nil {
				return
			}
		case <-ctx.Done():
			return
		default:
			return
		}
	}
}

// beat 按间隔发心跳; 回归窗口 (grace) 内无响应则释放连接.
func (p *peer) beat() {
	ticker := time.NewTicker(p.opts.heartbeat.interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.done:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(p.ctx, p.opts.heartbeat.grace)
			err := p.conn.Ping(ctx) // 阻塞直到 Pong 或超时
			cancel()
			if err != nil {
				_ = p.Close()
				return
			}
		}
	}
}

// Write 实现 bridge.Peer: 先按帧标准编码入队, 队列满时阻塞 (显式背压).
func (p *peer) Write(ctx context.Context, msg bridge.Message) error {
	if err := p.closedErr(); err != nil {
		return err
	}
	data, err := encodeFrame(msg)
	if err != nil {
		return err
	}
	if p.opts.maxMessageBytes > 0 && int64(len(data)) > p.opts.maxMessageBytes {
		return fmt.Errorf("ipc: outgoing message %d bytes exceeds max %d", len(data), p.opts.maxMessageBytes)
	}
	select {
	case p.out <- data:
		return nil
	case <-p.done:
		return errClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Read 实现 bridge.Peer: 读一帧并按帧标准解析.
func (p *peer) Read(ctx context.Context) (bridge.Message, error) {
	_, data, err := p.conn.Read(ctx)
	if err != nil {
		return bridge.Message{}, err
	}
	return decodeFrame(data)
}

// Close 实现 bridge.Peer (幂等): 先让写泵冲刷已入队的帧, 再关闭连接.
// 用 CloseNow 而非优雅关闭: 后者需要对端同时在读, 否则等满 ws 关闭握手超时, 拖住停机。
// 每一步都有界, 不依赖对端配合。
func (p *peer) Close() error {
	if p.cancel == nil {
		return p.conn.CloseNow()
	}
	p.once.Do(func() { close(p.drain) })
	select {
	case <-p.done:
	case <-time.After(closeFlushTimeout + time.Second): // 写泵兜底: 超时直接断
	}
	p.cancel()
	return p.conn.CloseNow()
}

// Local 实现 bridge.Peer.
func (p *peer) Local() string { return p.local }

// Remote 实现 bridge.Peer.
func (p *peer) Remote() string { return p.remote }

func (p *peer) closedErr() error {
	select {
	case <-p.done:
		return errClosed
	default:
		return nil
	}
}
