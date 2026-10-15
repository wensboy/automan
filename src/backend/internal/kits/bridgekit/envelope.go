package bridgekit

import (
	"github.com/wensboy/automan/backend/domain/bridge"
)

// EncodeMessage 把一条业务出站消息编成 ipc 消息: 业务负载 -> 信封 -> payload.
func EncodeMessage(codec bridge.Codec, out bridge.Outgoing) (bridge.Message, error) {
	data, err := codec.Encode(out.Data)
	if err != nil {
		return bridge.Message{}, err
	}
	payload, err := codec.Encode(bridge.Envelope{Kind: out.Kind, Name: out.Name, Data: data})
	if err != nil {
		return bridge.Message{}, err
	}
	return bridge.Message{ID: out.ID, Payload: payload}, nil
}

// DecodeMessage 把一条 ipc 消息解成业务包: payload -> 信封 -> Packet.
func DecodeMessage(codec bridge.Codec, msg bridge.Message) (bridge.Packet, error) {
	var env bridge.Envelope
	if err := codec.Decode(msg.Payload, &env); err != nil {
		return bridge.Packet{}, err
	}
	return bridge.Packet{
		ID:    msg.ID,
		Kind:  env.Kind,
		Name:  env.Name,
		Data:  env.Data,
		Codec: codec,
	}, nil
}
