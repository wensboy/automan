import type { Peer } from './client.ts';
import { type Outgoing, type Packet } from './protocol.ts';

/** 业务处理器: 返回要发出的消息(事件); 不回则 null; 也可以是 Promise(异步启动等场景)。 */
export type Handler = (packet: Packet) => Outgoing | null | Promise<Outgoing | null>;

/**
 * 单读循环: 按 name 把入站消息交给处理器。
 * 约定: 处理结果作为**事件**发出(与旧 stream 语义一致, 不自动关联请求 ID);
 *       未注册的 name 直接丢弃, 与 Go 侧一致。
 */
export function startLoop(peer: Peer, handlers: Record<string, Handler>): void {
  for (const [handlerName, handler] of Object.entries(handlers)) {
    peer.on(handlerName, (env) => {
      const packet: Packet = {
        id: env.id ?? '',
        kind: env.kind,
        name: env.name ?? handlerName,
        data: env.data,
      };
      Promise.resolve(handler(packet))
        .then((reply) => {
          if (reply === null || reply === undefined) return;
          if (env.kind === 'request' && env.id) {
            // 请求: 回 response(关联 ID), 由父进程的 Request 等待
            peer.respond(env.id, reply.name ?? packet.name, reply.data);
            return;
          }
          peer.notify(reply.name, reply.data);
        })
        .catch((err: unknown) => {
          if (env.kind === 'request' && env.id) {
            peer.fail(env.id, packet.name, String(err));
            return;
          }
          peer.notify(packet.name, { error: String(err) });
        });
    });
  }
}
