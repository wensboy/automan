import { protocolVersion, type Envelope, type Outgoing } from './protocol.ts';

/** 业务消息 -> 信封(直接 JSON, 不做 base64)。 */
export function encodeEnvelope(out: Outgoing): Envelope {
  const env: Envelope = { v: protocolVersion, kind: out.kind, name: out.name };
  if (out.id !== undefined && out.id !== '') env.id = out.id;
  if (out.data !== undefined) env.data = out.data;
  return env;
}

/** 文本帧 -> 信封(结构非法时抛错, 由调用方决定丢弃)。 */
export function decodeEnvelope(text: string): Envelope {
  const env = JSON.parse(text) as Envelope;
  if (env.kind === undefined) throw new Error('bad envelope: kind is required');
  return env;
}

/** 供业务读取 data 的辅助(未知时为 undefined)。 */
export function dataOf<T>(env: Envelope): T | undefined {
  return env.data as T | undefined;
}

/** 供业务读取 packet.data(兼容旧调用)。 */
export function decodeData<T>(packet: { data: unknown }): T {
  return packet.data as T;
}
