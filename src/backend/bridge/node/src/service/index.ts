import { decodeData } from '../ipc/envelope.ts';
import type { Handler } from '../ipc/loop.ts';
import { kind, type Outgoing } from '../ipc/protocol.ts';

/** 业务处理器表: 骨架阶段只放链路自检用的两个, 后续业务按 name 追加。 */
export const handlers: Record<string, Handler> = {
  // 链路自检: 原样回一条同 id 消息
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
