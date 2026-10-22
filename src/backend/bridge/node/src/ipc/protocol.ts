/**
 * v3 线协议: 文本帧内容**直接是信封 JSON**(不再套 base64 帧), 见 docs/design/v23.md §4。
 * 与 Go 侧 message.Envelope 字段逐字一致。
 */
export const protocolVersion = 3;

export const kind = {
  request: 'request',
  response: 'response',
  error: 'error',
  event: 'event',
  /** 业务侧沿用的别名: Handler 返回对象里的占位, loop 会以事件发出(不再有线上的 stream 语义)。 */
  stream: 'stream',
} as const;

export type Kind = (typeof kind)[keyof typeof kind];

export const name = {
  handshake: 'bridge.handshake',
  ready: 'node.ready',
  log: 'node.log',
} as const;

/** 文本帧内容(信封)。 */
export interface Envelope {
  v: number;
  kind: Kind;
  name?: string;
  id?: string;
  data?: unknown;
}

/** 交给业务处理的一条消息。 */
export interface Packet {
  id: string;
  kind: Kind;
  name: string;
  data: unknown;
}

/** 业务要发出的一条消息。 */
export interface Outgoing {
  id?: string;
  kind: Kind;
  name: string;
  data?: unknown;
}
