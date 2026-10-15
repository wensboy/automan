package ipc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/wensboy/automan/backend/domain/bridge"
)

// 帧标准 (ipc 层的统一标准, 选定后不变): 固定的 json 结构
//
//	{"id":"<关联 id>","payload":"<base64>"}
//
// payload 是上层给的不透明字节 (业务负载编码结果), 因此用 base64 承载 ——
// 传输层不假设其格式, 业务侧以后更换编码实现也不需要改传输。
type frame struct {
	ID      string `json:"id"`
	Payload string `json:"payload,omitempty"`
}

func encodeFrame(msg bridge.Message) ([]byte, error) {
	data, err := json.Marshal(frame{ID: msg.ID, Payload: base64.StdEncoding.EncodeToString(msg.Payload)})
	if err != nil {
		return nil, fmt.Errorf("ipc: encode frame: %w", err)
	}
	return data, nil
}

func decodeFrame(data []byte) (bridge.Message, error) {
	var f frame
	if err := json.Unmarshal(data, &f); err != nil {
		return bridge.Message{}, fmt.Errorf("ipc: decode frame: %w", err)
	}
	payload, err := base64.StdEncoding.DecodeString(f.Payload)
	if err != nil {
		return bridge.Message{}, fmt.Errorf("ipc: decode payload: %w", err)
	}
	return bridge.Message{ID: f.ID, Payload: payload}, nil
}
