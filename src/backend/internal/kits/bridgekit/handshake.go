package bridgekit

import (
	"context"
	"errors"
	"fmt"

	"github.com/wensboy/automan/backend/domain/bridge"
)

// ClientHandshake 客户端侧握手: 发出握手消息并等待应答.
func ClientHandshake(ctx context.Context, p bridge.Peer, codec bridge.Codec, req bridge.Handshake) (bridge.HandshakeResult, error) {
	msg, err := EncodeMessage(codec, bridge.Outgoing{
		ID:   bridge.NameHandshake,
		Kind: bridge.KindStream,
		Name: bridge.NameHandshake,
		Data: req,
	})
	if err != nil {
		return bridge.HandshakeResult{}, err
	}
	if err := p.Write(ctx, msg); err != nil {
		return bridge.HandshakeResult{}, err
	}
	raw, err := p.Read(ctx)
	if err != nil {
		return bridge.HandshakeResult{}, err
	}
	packet, err := DecodeMessage(codec, raw)
	if err != nil {
		return bridge.HandshakeResult{}, err
	}
	if packet.Kind == bridge.KindError {
		return bridge.HandshakeResult{}, fmt.Errorf("bridge: handshake rejected: %s", packet.Data)
	}
	if packet.Name != bridge.NameHandshake {
		return bridge.HandshakeResult{}, fmt.Errorf("bridge: unexpected handshake reply %q", packet.Name)
	}
	var result bridge.HandshakeResult
	if err := packet.Decode(&result); err != nil {
		return bridge.HandshakeResult{}, err
	}
	return result, nil
}

// ServerHandshake 服务端侧握手: 读取并校验对端握手, 回送应答.
func ServerHandshake(ctx context.Context, p bridge.Peer, codec bridge.Codec, want bridge.Handshake) (bridge.Handshake, error) {
	raw, err := p.Read(ctx)
	if err != nil {
		return bridge.Handshake{}, err
	}
	packet, err := DecodeMessage(codec, raw)
	var got bridge.Handshake
	if err == nil {
		err = packet.Decode(&got)
	}
	if err == nil {
		err = validate(packet, got, want)
	}
	if err != nil {
		reason, _ := codec.Encode(err.Error())
		reply, encErr := codec.Encode(bridge.Envelope{
			Kind: bridge.KindError,
			Name: bridge.NameHandshake,
			Data: reason,
		})
		if encErr == nil {
			_ = p.Write(ctx, bridge.Message{ID: raw.ID, Payload: reply})
		}
		return bridge.Handshake{}, err
	}
	reply, err := EncodeMessage(codec, bridge.Outgoing{
		ID:   raw.ID,
		Kind: bridge.KindStream,
		Name: bridge.NameHandshake,
		Data: bridge.HandshakeResult{Daemon: want.Daemon, Codec: want.Codec, Version: want.Version},
	})
	if err != nil {
		return bridge.Handshake{}, err
	}
	if err := p.Write(ctx, reply); err != nil {
		return bridge.Handshake{}, err
	}
	return got, nil
}

// validate 校验握手 (名称/编码/版本/凭据); codec 名只作断言, 不协商.
func validate(packet bridge.Packet, got bridge.Handshake, want bridge.Handshake) error {
	switch {
	case packet.Name != bridge.NameHandshake:
		return fmt.Errorf("bridge: expected %q, got %q", bridge.NameHandshake, packet.Name)
	case got.Codec != want.Codec:
		return fmt.Errorf("bridge: codec mismatch: want %q, got %q", want.Codec, got.Codec)
	case got.Version != want.Version:
		return fmt.Errorf("bridge: version mismatch: want %d, got %d", want.Version, got.Version)
	case want.Token != "" && got.Token != want.Token:
		return errors.New("bridge: token mismatch")
	}
	return nil
}
