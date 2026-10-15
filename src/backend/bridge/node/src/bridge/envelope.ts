import type { Envelope, Outgoing, Packet } from './protocol.ts';

/** 业务负载 -> 信封 -> payload 字节 (对应 Go 侧 bridgekit.EncodeMessage)。 */
export function encodeMessage(out: Outgoing): { id: string; payload: Uint8Array } {
  const envelope: Envelope = { kind: out.kind, name: out.name };
  if (out.data !== undefined) {
    const data = Buffer.from(JSON.stringify(out.data));
    if (data.length > 0) envelope.data = data.toString('base64');
  }
  return { id: out.id ?? '', payload: Buffer.from(JSON.stringify(envelope)) };
}

/** payload 字节 -> 信封 -> Packet (对应 Go 侧 bridgekit.DecodeMessage)。 */
export function decodeMessage(id: string, payload: Uint8Array): Packet {
  const envelope = JSON.parse(Buffer.from(payload).toString('utf8')) as Envelope;
  return {
    id,
    kind: envelope.kind,
    name: envelope.name ?? '',
    data: envelope.data ? Buffer.from(envelope.data, 'base64') : new Uint8Array(),
  };
}

/** 解码业务负载 (当前约定为 json)。 */
export function decodeData<T>(packet: Packet): T {
  return JSON.parse(Buffer.from(packet.data).toString('utf8')) as T;
}
