package ipc

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/coder/websocket"

	"github.com/wensboy/automan/backend/domain/bridge"
)

// dialURL 的 host 无意义: 实际连接由 unix dialer 决定.
const dialURL = "ws://bridge/"

// serve 在已绑定的 unix listener 上启动 ws 服务端与接受循环.
func serve(ln net.Listener, o options, addr string) *Server {
	s := &Server{
		addr:  addr,
		ln:    ln,
		peers: make(chan bridge.Peer),
		done:  make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { s.handle(w, r, o) })
	s.srv = &http.Server{Handler: mux, ReadHeaderTimeout: o.upgradeTimeout}
	go func() { _ = s.srv.Serve(ln) }()
	return s
}

// handle 完成 ws 升级并交出一条连接.
func (s *Server) handle(w http.ResponseWriter, r *http.Request, o options) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	conn.SetReadLimit(o.maxMessageBytes)
	p := newPeer(conn, o, s.addr, r.RemoteAddr)
	p.start()
	select {
	case s.peers <- p:
	case <-s.done:
		_ = p.Close()
	}
}

// dial 建立主动连接 (unix socket + ws 升级) 并启动 peer 内部循环.
func dial(ctx context.Context, target string, o options) (bridge.Peer, error) {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", target)
			},
		},
	}
	conn, resp, err := websocket.Dial(ctx, dialURL, &websocket.DialOptions{HTTPClient: client})
	if err != nil {
		return nil, fmt.Errorf("ipc: dial %s: %w", target, err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	conn.SetReadLimit(o.maxMessageBytes)
	p := newPeer(conn, o, target, target)
	p.start()
	return p, nil
}
