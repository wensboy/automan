import type WebSocket from 'ws';

import { nextPacket, send } from './client.ts';
import { decodeData } from './envelope.ts';
import { kind, name, protocolVersion } from './protocol.ts';

export interface HandshakeOptions {
  daemon: string;
  codec: string;
  token: string;
}

interface HandshakeResult {
  daemon: string;
  codec: string;
  version: number;
}

/** 客户端侧握手: 进入消息循环前的一次性交换 (codec 仅作断言, 不协商)。 */
export async function handshake(ws: WebSocket, options: HandshakeOptions): Promise<HandshakeResult> {
  send(ws, {
    id: name.handshake,
    kind: kind.stream,
    name: name.handshake,
    data: {
      daemon: options.daemon,
      codec: options.codec,
      version: protocolVersion,
      token: options.token,
    },
  });

  const reply = await nextPacket(ws);
  if (reply.kind === kind.error) {
    throw new Error(`handshake rejected: ${decodeData<string>(reply)}`);
  }
  if (reply.name !== name.handshake) {
    throw new Error(`unexpected handshake reply: ${reply.name}`);
  }

  const result = decodeData<HandshakeResult>(reply);
  if (result.version !== protocolVersion) {
    throw new Error(`version mismatch: want ${protocolVersion}, got ${result.version}`);
  }
  if (result.codec !== options.codec) {
    throw new Error(`codec mismatch: want ${options.codec}, got ${result.codec}`);
  }
  return result;
}
