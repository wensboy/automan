package bridge

import (
	stdctx "context"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/internal/kits/bridgekit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// serve 接受连接并逐条建立会话 (单连接策略: 同一时刻只服务一个对端).
func (e *Engine) serve(ctx stdctx.Context) {
	defer e.wg.Done()
	for {
		if ctx.Err() != nil {
			return
		}
		server := e.currentServer()
		if server == nil {
			return
		}
		peer, err := server.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			e.logWarn("bridge: accept failed", logkit.F("err", err))
			// 恢复: 重绑监听后继续接受.
			if rebindErr := e.rebind(); rebindErr != nil {
				e.logError("bridge: rebind failed", logkit.F("err", rebindErr))
				return
			}
			continue
		}
		e.attach(ctx, peer)
	}
}

// attach 完成握手并进入会话; 会话结束后做恢复 (重绑 + 继续接受).
func (e *Engine) attach(ctx stdctx.Context, peer domainbridge.Peer) {
	if _, err := e.handshake(ctx, peer); err != nil {
		e.logError("bridge: handshake failed",
			logkit.F("remote", peer.Remote()), logkit.F("err", err))
		_ = peer.Close()
		return
	}
	e.setPeer(peer)
	e.logInfo("bridge: connected", logkit.F("remote", peer.Remote()))
	e.emit(event.TopicBridgeConnected, event.BridgeConnected{Daemon: e.daemon, Remote: peer.Remote()})

	err := e.repl(ctx, peer)

	e.setPeer(nil)
	_ = peer.Close()
	e.logInfo("bridge: disconnected",
		logkit.F("remote", peer.Remote()), logkit.F("err", err))
	e.emit(event.TopicBridgeDisconnected, event.BridgeDisconnected{
		Daemon: e.daemon,
		Remote: peer.Remote(),
		Err:    errText(err),
	})

	// 会话结束不重绑: 监听仍然有效, 请对端直接重连 (重绑只用于接受失败与子进程退出).
}

// handshake 会话建立后、进入分派前的一次性自检 (不协商编码).
func (e *Engine) handshake(ctx stdctx.Context, peer domainbridge.Peer) (domainbridge.Handshake, error) {
	want := domainbridge.Handshake{
		Daemon:  e.daemon,
		Codec:   e.codecName,
		Version: domainbridge.ProtocolVersion,
		Token:   e.token,
	}
	if e.handshakeTimeout <= 0 {
		return bridgekit.ServerHandshake(ctx, peer, e.codec, want)
	}
	hsCtx, cancel := stdctx.WithTimeout(ctx, e.handshakeTimeout)
	defer cancel()
	return bridgekit.ServerHandshake(hsCtx, peer, e.codec, want)
}

// repl 是单读循环: 收到的每条消息都按 name 分派 (kind 属业务层, 引擎不解释).
func (e *Engine) repl(ctx stdctx.Context, peer domainbridge.Peer) error {
	for {
		msg, err := peer.Read(ctx)
		if err != nil {
			return err
		}
		e.dispatch(ctx, msg)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
