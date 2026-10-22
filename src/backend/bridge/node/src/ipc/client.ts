import { connect as netConnect } from 'node:net';
import WebSocket from 'ws';

import { decodeEnvelope, encodeEnvelope } from './envelope.ts';
import { kind, type Envelope, type Outgoing } from './protocol.ts';

/** 连接 uds: 用 createConnection 让 ws 走 unix socket(不监听/不连接 TCP)。 */
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

type Pending = {
  resolve: (env: Envelope) => void;
  reject: (err: Error) => void;
  timer: NodeJS.Timeout;
};

/**
 * Peer: 请求/响应关联 + 事件发送 + 处理器注册 + 心跳。
 * 与 Go 侧 message.Peer 对称(两侧都可发请求, 见 v23 §12 C)。
 */
export class Peer {
  private readonly pending = new Map<string, Pending>();
  private readonly handlers = new Map<string, (env: Envelope) => void>();
  private seq = 0;
  private closed = false;

  constructor(private readonly ws: WebSocket, private readonly pingMs = 20000) {
    ws.on('message', (data: WebSocket.RawData) => this.onMessage(data.toString()));
    ws.on('close', () => this.failAll('connection closed'));
    ws.on('error', (err: Error) => this.failAll(String(err)));
    if (pingMs > 0) {
      const timer = setInterval(() => {
        try {
          ws.ping();
        } catch {
          // 已断开: 由 close 事件收尾
        }
      }, pingMs);
      ws.on('close', () => clearInterval(timer));
    }
  }

  /** 注册处理器: 对端发来的 request/event 按 name 分发。 */
  on(name: string, handler: (env: Envelope) => void): void {
    this.handlers.set(name, handler);
  }

  /** 回应对端的请求(关联 ID)。 */
  respond(id: string, name: string, data?: unknown): void {
    this.send({ id, kind: kind.response, name, data });
  }

  /** 回应对端请求的失败(error 帧, 关联 ID)。 */
  fail(id: string, name: string, message: string): void {
    this.send({ id, kind: kind.error, name, data: { code: 'handler_error', message } });
  }

  /** 单向事件。 */
  notify(name: string, data?: unknown): void {
    this.send({ kind: kind.event, name, data });
  }

  /** 请求并等待响应(超时/关闭都 reject)。 */
  request(name: string, data: unknown, timeoutMs = 30000): Promise<Envelope> {
    const id = `${Date.now().toString(16)}${(this.seq++).toString(16)}`;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`request timeout: ${name}`));
      }, timeoutMs);
      this.pending.set(id, { resolve, reject, timer });
      try {
        this.send({ id, kind: kind.request, name, data });
      } catch (err) {
        clearTimeout(timer);
        this.pending.delete(id);
        reject(err instanceof Error ? err : new Error(String(err)));
      }
    });
  }

  close(): void {
    this.closed = true;
    try {
      this.ws.close(1000);
    } catch {
      // 已关闭
    }
    this.failAll('peer closed');
  }

  private send(out: Outgoing): void {
    if (this.closed) throw new Error('peer closed');
    this.ws.send(JSON.stringify(encodeEnvelope(out)));
  }

  private onMessage(text: string): void {
    let env: Envelope;
    try {
      env = decodeEnvelope(text);
    } catch {
      return; // 非法帧: 丢弃(协议错误由 Go 侧决定是否断开)
    }
    if (env.kind === kind.response || env.kind === kind.error) {
      const waiter = env.id ? this.pending.get(env.id) : undefined;
      if (waiter) {
        this.pending.delete(env.id as string);
        clearTimeout(waiter.timer);
        if (env.kind === kind.error) {
          const detail = env.data as { message?: string; code?: string } | undefined;
          waiter.reject(new Error(detail?.message ?? 'remote error'));
        } else {
          waiter.resolve(env);
        }
      }
      return;
    }

    const handler = this.handlers.get(env.name ?? '');
    if (handler === undefined) {
      if (env.kind === kind.request && env.id) {
        this.send({ id: env.id, kind: kind.error, name: env.name ?? '', data: { code: 'unregistered', message: `no handler for ${env.name}` } });
      }
      return;
    }

    // 处理器**自己负责**回写(loop 里按 request/event 分流):
    // 这里不再自动应答, 否则会与 loop 的响应重复(曾导致 Go 侧先收到一个 data=null 的响应)。
    handler(env);
  }

  private failAll(reason: string): void {
    for (const [id, waiter] of this.pending) {
      clearTimeout(waiter.timer);
      waiter.reject(new Error(reason));
      this.pending.delete(id);
    }
  }
}

/** 重连退避: 指数增长, 上限 maxMs。 */
export function backoffMs(attempt: number, baseMs: number, maxMs: number): number {
  if (attempt <= 0) return baseMs;
  return Math.min(baseMs * 2 ** (attempt - 1), maxMs);
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
