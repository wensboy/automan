// Package bridge 定义父子进程 IPC 的契约: 传输消息 (帧内容) + 桥接协议消息.
package bridge

// Message 是 ipc 帧的内容: 只有传输规范要求的字段.
type Message struct {
	ID      string // 连接内多路寻址 (帧归属); 传输只透传, 不解释
	Payload []byte // 不透明字节 (= 业务负载编码结果, 传输不解析)
}
