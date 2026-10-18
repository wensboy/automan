import { decodeData } from '../ipc/envelope.ts';
import type { Handler } from '../ipc/loop.ts';
import { kind, type Outgoing } from '../ipc/protocol.ts';

/** 链路自检处理器: 回显与延迟探针(不涉及业务)。 */
export const healthHandlers: Record<string, Handler> = {
  // 原样回一条同 id 消息
  'node.echo': (packet): Outgoing => ({
    id: packet.id,
    kind: kind.stream,
    name: packet.name,
    data: decodeData<unknown>(packet),
  }),

  // 连通性与延迟自检
  'node.ping': (packet): Outgoing => ({
    id: packet.id,
    kind: kind.stream,
    name: 'node.pong',
    data: { at: Date.now() },
  }),
};
