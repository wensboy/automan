package bridgekit

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/wensboy/automan/backend/domain/bridge"
)

// JSON 是内置负载编解码: 在信封之上解释负载, 供桥接协议层使用.
type JSON struct{}

// Name 实现 bridge.Codec.
func (JSON) Name() string { return "json" }

// Encode 实现 bridge.Codec.
func (JSON) Encode(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("bridge: encode payload: %w", err)
	}
	return data, nil
}

// Decode 实现 bridge.Codec.
func (JSON) Decode(data []byte, into any) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("bridge: decode payload: %w", err)
	}
	return nil
}

var (
	codecMu sync.RWMutex
	codecs  = map[string]bridge.Codec{}
)

// RegisterCodec 注册负载编解码实现 (按名称, 供握手协商与按名选择).
func RegisterCodec(c bridge.Codec) {
	codecMu.Lock()
	defer codecMu.Unlock()
	codecs[c.Name()] = c
}

// Codec 按名称取负载编解码实现.
func Codec(name string) (bridge.Codec, bool) {
	codecMu.RLock()
	defer codecMu.RUnlock()
	c, ok := codecs[name]
	return c, ok
}

func init() { RegisterCodec(JSON{}) }
