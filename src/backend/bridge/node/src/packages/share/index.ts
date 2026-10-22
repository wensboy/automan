/** 共享工具: 允许在 ESM 产物里使用 CJS 的 require(打包时由 banner 注入)。 */
declare const require: (id: string) => any;

/** proxy 子包与 service 层共用的小工具(不含 whistle 细节)。 */

/** 把毫秒超时归一到合理区间。 */
export function clampTimeout(value: number | undefined, fallback: number, max = 120000): number {
  if (!value || value <= 0) return fallback;
  return Math.min(value, max);
}

/** 端口是否被占用(极简探测, 只用于 stop 后的校验)。 */
export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** 统一的错误文本。 */
export function messageOf(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === 'string') return err;
  try {
    return JSON.stringify(err);
  } catch {
    return String(err);
  }
}

/** 单个端口是否可连(用于"是否真的起来/是否真的停掉"的判定)。 */
export function portListening(host: string, port: number, timeoutMs = 500): Promise<boolean> {
  const net = require('node:net') as typeof import('node:net');
  return new Promise((resolve) => {
    const socket = net.connect({ host, port });
    const done = (ok: boolean): void => {
      socket.removeAllListeners();
      socket.destroy();
      resolve(ok);
    };
    socket.setTimeout(timeoutMs);
    socket.once('connect', () => done(true));
    socket.once('timeout', () => done(false));
    socket.once('error', () => done(false));
  });
}

/** 端口 -> pid(Linux: ss, macOS: lsof); 取不到返回空。 */
function pidsByPort(port: number): number[] {
  const cp = require('node:child_process') as typeof import('node:child_process');
  const run = (cmd: string, args: string[]): string => {
    try {
      return String(cp.execFileSync(cmd, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }));
    } catch {
      return '';
    }
  };
  const pids = new Set<number>();
  for (const line of run('ss', ['-ltnp']).split('\n')) {
    if (!line.includes(`:${port} `) && !line.includes(`:${port}\t`)) continue;
    for (const match of line.matchAll(/pid=(\d+)/g)) pids.add(Number(match[1]));
  }
  if (pids.size === 0) {
    for (const token of run('lsof', ['-ti', `tcp:${port}`]).split('\n')) {
      const pid = Number(token.trim());
      if (Number.isFinite(pid) && pid > 0) pids.add(pid);
    }
  }
  return [...pids];
}

/** 结束监听指定端口的进程: TERM -> 等 -> KILL。 */
export async function killByPort(port: number, timeoutMs = 3000): Promise<void> {
  for (const pid of pidsByPort(port)) {
    try {
      process.kill(pid, 'SIGTERM');
    } catch {
      // 已退出
    }
  }
  const deadline = Date.now() + Math.max(500, Math.floor(timeoutMs / 2));
  while (Date.now() < deadline) {
    if (pidsByPort(port).length === 0) return;
    await sleep(150);
  }
  for (const pid of pidsByPort(port)) {
    try {
      process.kill(pid, 'SIGKILL');
    } catch {
      // 已退出
    }
  }
  await sleep(200);
}
