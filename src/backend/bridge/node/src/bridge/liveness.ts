import type WebSocket from 'ws';

/**
 * 主动探活: 按间隔 ping 父进程; 超过 grace (2 倍间隔) 未收到 pong 视为父进程假死,
 * 主动断开以触发重连。
 */
export function startLiveness(ws: WebSocket, intervalMs: number): () => void {
  if (intervalMs <= 0) return () => {};

  const graceMs = intervalMs * 2;
  let lastPong = Date.now();
  ws.on('pong', () => {
    lastPong = Date.now();
  });

  const timer = setInterval(() => {
    if (Date.now() - lastPong > graceMs) {
      ws.terminate();
      return;
    }
    ws.ping();
  }, intervalMs);

  return () => clearInterval(timer);
}
