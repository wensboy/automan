/** 协议常量与类型: 必须与 Go 侧 domain/bridge 逐字一致 (否则对端无法解码)。 */

export const protocolVersion = 2;

export const kind = {
  stream: 'stream',
  event: 'event',
  error: 'error',
} as const;

export type Kind = (typeof kind)[keyof typeof kind];

export const name = {
  handshake: 'bridge.handshake',
  ready: 'node.ready',
  log: 'node.log',
} as const;

/** 帧: ws 文本帧的内容 (对应 Go 侧 ipc/frame.go)。 */
export interface Frame {
  id: string;
  payload?: string;
}

/** 信封: payload 字节的 json (对应 Go 侧 bridge.Envelope)。 */
export interface Envelope {
  kind: Kind;
  name?: string;
  data?: string;
}

/** 交给业务处理的一条消息 (对应 Go 侧 bridge.Packet)。 */
export interface Packet {
  id: string;
  kind: Kind;
  name: string;
  data: Uint8Array;
}

/** 业务要发出的一条消息 (对应 Go 侧 bridge.Outgoing)。 */
export interface Outgoing {
  id?: string;
  kind: Kind;
  name: string;
  data?: unknown;
}
