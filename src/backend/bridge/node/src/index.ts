import { backoffMs, Peer, connect, sleep } from './ipc/client.ts';
import { handshake } from './ipc/handshake.ts';
import { log } from './ipc/log.ts';
import { startLoop } from './ipc/loop.ts';
import { parseConfig, type Config } from './config.ts';
import { handlers } from './service/index.ts';

/** 一次会话: 连接 -> 握手 -> 就绪通知 -> 读循环, 直到连接结束。 */
async function session(config: Config, attempt: number): Promise<void> {
  const ws = await connect(config.sock);
  const peer = new Peer(ws, config.livenessMs);
  const result = await handshake(peer, {
    daemon: config.daemon,
    codec: config.codec,
    token: config.token,
  });
  log(peer, 'info', 'bridge: connected', { sock: config.sock, daemon: result.daemon, attempt });
  // 就绪通知(事件): 父进程据此更新状态
  peer.notify('node.ready', { pid: process.pid, sock: config.sock });

  startLoop(peer, handlers);
  try {
    await new Promise<void>((resolve) => {
      ws.once('close', (code: number, reason: Buffer) => {
        log(undefined, 'warn', 'bridge: closed', { code, reason: reason.toString() });
        resolve();
      });
      ws.once('error', () => resolve());
    });
  } finally {
    peer.close();
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
