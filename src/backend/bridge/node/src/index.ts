import { backoffMs, closed, connect, send, sleep } from './ipc/client.ts';
import { handshake } from './ipc/handshake.ts';
import { startLiveness } from './ipc/liveness.ts';
import { log } from './ipc/log.ts';
import { startLoop } from './ipc/loop.ts';
import { kind, name } from './ipc/protocol.ts';
import { parseConfig, type Config } from './config.ts';
import { handlers } from './service/index.ts';

/** 一次会话: 连接 -> 握手 -> 就绪通知 -> 读循环, 直到连接结束。 */
async function session(config: Config, attempt: number): Promise<void> {
  const ws = await connect(config.sock);
  const result = await handshake(ws, {
    daemon: config.daemon,
    codec: config.codec,
    token: config.token,
  });
  log(ws, 'info', 'bridge: connected', { sock: config.sock, daemon: result.daemon, attempt });
  send(ws, { kind: kind.event, name: name.ready, data: { pid: process.pid, sock: config.sock } });

  startLoop(ws, handlers);
  const stopLiveness = startLiveness(ws, config.livenessMs);
  try {
    ws.on('close', (code: number, reason: Buffer) => {
      log(undefined, 'warn', 'bridge: closed', { code, reason: reason.toString() });
    });
    await closed(ws);
  } finally {
    stopLiveness();
  }
}

async function main(): Promise<void> {
  const config = parseConfig(process.argv.slice(2));
  let attempt = 0;

  for (;;) {
    try {
      await session(config, attempt);
      attempt = 0;
      log(undefined, 'warn', 'bridge: disconnected, reconnecting');
    } catch (err) {
      attempt += 1;
      log(undefined, 'warn', 'bridge: session failed', { attempt, err: String(err) });
      if (config.reconnectAttempts > 0 && attempt > config.reconnectAttempts) {
        log(undefined, 'error', 'bridge: give up', { attempts: attempt });
        process.exit(1);
      }
    }
    await sleep(backoffMs(attempt, config.reconnectBaseMs, config.reconnectMaxMs));
  }
}

main().catch((err: unknown) => {
  console.error(err);
  process.exit(1);
});
