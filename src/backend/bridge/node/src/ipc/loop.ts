import type WebSocket from 'ws';

import { send } from './client.ts';
import { decodeMessage } from './envelope.ts';
import { decodeFrame } from './frame.ts';
import { log } from './log.ts';
import { kind, type Outgoing, type Packet } from './protocol.ts';

/** 业务处理器: 返回要发出的消息; 不回则返回 null; 也可以是 Promise(异步启动等场景)。 */
export type Handler = (packet: Packet) => Outgoing | null | Promise<Outgoing | null>;

/** 单读循环: 收帧 -> 解信封 -> 按 name 分派; 未注册 name 丢弃 (与 Go 侧一致)。 */
export function startLoop(ws: WebSocket, handlers: Record<string, Handler>): void {
  ws.on('message', (data: WebSocket.RawData) => {
    let packet: Packet;
    try {
      const frame = decodeFrame(data.toString());
      packet = decodeMessage(frame.id, frame.payload);
    } catch (err) {
      log(ws, 'warn', 'bridge: decode failed', { err: String(err) });
      return;
    }

    const handler = handlers[packet.name];
    if (handler === undefined) {
      log(ws, 'warn', 'bridge: message dropped', { reason: 'unregistered-name', name: packet.name });
      return;
    }

    const respond = (reply: Outgoing | null) => {
      if (reply !== null) send(ws, reply);
    };
    const fail = (err: unknown) => {
      log(ws, 'error', 'bridge: handler failed', { name: packet.name, err: String(err) });
      send(ws, {
        id: packet.id,
        kind: kind.error,
        name: packet.name,
        data: String(err),
      });
    };

    try {
      const result = handler(packet);
      // 异步处理器(proxy.start 等)在此等待, 但**不阻塞**后续消息的分发
      if (result instanceof Promise) {
        void result.then(respond).catch(fail);
      } else {
        respond(result);
      }
    } catch (err) {
      fail(err);
    }
  });
}
