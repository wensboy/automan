package config

import "github.com/wensboy/automan/backend/domain/key"

// Bridge 映射 data/conf/bridge.json 中单个桥接 daemon 的参数.
type Bridge struct {
	Enable             *bool    `json:"enable"` // nil 视为启用
	Sock               string   `json:"sock"`   // 相对存储根, 缺省 bridge/<daemon>.sock
	Codec              string   `json:"codec"`  // 业务负载编码名 (与 ipc 帧标准相互独立)
	MaxMessageBytes    int64    `json:"maxMessageBytes"`
	HandshakeTimeoutMs int      `json:"handshakeTimeoutMs"`
	HeartbeatMs        int      `json:"heartbeatMs"`      // 0 关闭心跳
	HeartbeatGraceMs   int      `json:"heartbeatGraceMs"` // 回归窗口: 未回 Pong 则释放连接
	Token              string   `json:"token"`
	Restart            string   `json:"restart"` // none | on-fail | always
	RestartMax         int      `json:"restartMax"`
	Command            string   `json:"command"` // 空 -> 附着模式 (子进程由外部启动)
	Args               []string `json:"args"`
	Dir                string   `json:"dir"`
	Env                []string `json:"env"`
	// Services 是该 daemon 承载的各业务服务的专属参数(当前只有 proxy/automation).
	Services BridgeServices `json:"services"`
}

// BridgeServices 是 daemon 下按服务划分的参数.
type BridgeServices struct {
	Proxy struct {
		Root          string `json:"root"`          // 代理 basedir(相对存储根)
		StopTimeoutMs int    `json:"stopTimeoutMs"` // 停止强杀阈值
	} `json:"proxy"`
	Automation map[string]any `json:"automation"`
}

// BridgeParams 是 conf/bridge 命名空间的整棵对象: daemon 名 -> 参数.
type BridgeParams map[string]Bridge

// BridgeKey 取 conf/bridge 命名空间根.
var BridgeKey = key.New[BridgeParams]("conf/bridge")
