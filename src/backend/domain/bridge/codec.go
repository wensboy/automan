package bridge

// Codec 是负载编解码: 位于信封之上, 由桥接协议层使用, 传输层不感知.
// 每种负载格式有名称 (用于握手协商与按名选择); 进程级固定, 不按连接切换.
type Codec interface {
	Name() string
	Encode(v any) ([]byte, error)
	Decode(data []byte, into any) error
}
