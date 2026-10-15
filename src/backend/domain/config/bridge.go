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
}

// BridgeParams 是 conf/bridge 命名空间的整棵对象: daemon 名 -> 参数.
type BridgeParams map[string]Bridge

// BridgeKey 取 conf/bridge 命名空间根.
var BridgeKey = key.New[BridgeParams]("conf/bridge")
