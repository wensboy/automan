import WebSocket from 'ws';

import { send } from './client.ts';
import { kind, name } from './protocol.ts';

/**
 * 统一日志: 本地写 stdout/stderr (父进程继承可见), 同时经 kind=event 上行给父进程。
 * ws 为空表示尚未连接 (仅在本地输出)。
 */
export function log(
  ws: WebSocket | undefined,
  level: string,
  msg: string,
  fields?: Record<string, unknown>,
): void {
  const line = fields === undefined ? msg : `${msg} ${JSON.stringify(fields)}`;
  if (level === 'error' || level === 'warn') {
    console.error(line);
  } else {
    console.log(line);
  }
  if (ws === undefined || ws.readyState !== WebSocket.OPEN) return;
  try {
    send(ws, { kind: kind.event, name: name.log, data: { level, msg, fields } });
  } catch {
    // 日志上行失败不影响本地输出
  }
}
