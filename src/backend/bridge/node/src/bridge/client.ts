import { connect as netConnect } from 'node:net';
import WebSocket from 'ws';

import { decodeMessage, encodeMessage } from './envelope.ts';
import { decodeFrame, encodeFrame } from './frame.ts';
import type { Outgoing, Packet } from './protocol.ts';

/** 连接 uds: 用 createConnection 让 ws 走 unix socket (不监听/不连接 TCP)。 */
export function connect(sockPath: string): Promise<WebSocket> {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket('ws://bridge/', {
      createConnection: () => netConnect({ path: sockPath }),
      perMessageDeflate: false,
    });
    ws.once('open', () => resolve(ws));
    ws.once('error', reject);
  });
}

/** 发送一条业务消息。 */
export function send(ws: WebSocket, out: Outgoing): void {
  const { id, payload } = encodeMessage(out);
  ws.send(encodeFrame(id, payload));
}

/** 读取下一条消息: 仅握手期一次性使用 (进入循环后由 loop 接管 message 事件)。 */
export function nextPacket(ws: WebSocket): Promise<Packet> {
  return new Promise((resolve, reject) => {
    const cleanup = (): void => {
      ws.off('message', onMessage);
      ws.off('close', onClose);
      ws.off('error', onError);
    };
    const onMessage = (data: WebSocket.RawData): void => {
      cleanup();
      try {
        const frame = decodeFrame(data.toString());
        resolve(decodeMessage(frame.id, frame.payload));
      } catch (err) {
        reject(err instanceof Error ? err : new Error(String(err)));
      }
    };
    const onClose = (): void => {
      cleanup();
      reject(new Error('connection closed'));
    };
    const onError = (err: Error): void => {
      cleanup();
      reject(err);
    };
    ws.on('message', onMessage);
    ws.once('close', onClose);
    ws.once('error', onError);
  });
}

/** 等待连接结束 (正常关闭或出错)。 */
export function closed(ws: WebSocket): Promise<void> {
  return new Promise((resolve) => {
    ws.once('close', () => resolve());
    ws.once('error', () => resolve());
  });
}

/** 重连退避: 指数增长, 上限 maxMs。 */
export function backoffMs(attempt: number, baseMs: number, maxMs: number): number {
  if (attempt <= 0) return baseMs;
  return Math.min(baseMs * 2 ** (attempt - 1), maxMs);
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
