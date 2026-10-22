import type { Peer } from './client.ts';

/** 把一条日志上报给父进程(事件 node.log); peer 未就绪时退化为 stderr。 */
export function log(peer: Peer | undefined, level: 'info' | 'warn' | 'error', msg: string, fields?: Record<string, unknown>): void {
  if (peer === undefined) {
    console.error(`[${level}] ${msg}`, fields ?? {});
    return;
  }
  try {
    peer.notify('node.log', { level, msg, fields });
  } catch {
    console.error(`[${level}] ${msg}`, fields ?? {});
  }
}
