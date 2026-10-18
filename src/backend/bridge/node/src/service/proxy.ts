/**
 * 代理执行链路: 每条代理一个 whistle 实例。
 *
 * 启动方式: 优先用 whistle 自带的进程管理库 `starting`(`cli.start/stop`) —— 即官方
 * "standalone service mode": 传入 whistle 的入口与配置, 由它按配置启动并在
 * `~/.startingAppData` 记录 pid, 停止时可精确回收(见 docs/design/v19.md §26)。
 * 其中 `main` 允许带 `#hash` 作为**实例隔离键**(starting 内部用它区分配置文件, spawn 前剥掉),
 * 因此每条代理用 `<whistle index.js>#<proxyId>` 即可互不干扰。
 *
 * 兜底: 若 `starting` 不可解析(例如只设置了 WHISTLE_BIN 的精简部署), 退回
 * 直接 spawn whistle CLI, 停止时用"wrapper 信号 + 端口反查"。
 */
import { spawn, type ChildProcess } from 'node:child_process';
import { existsSync, mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

import { decodeData } from '../ipc/envelope.ts';
import type { Handler } from '../ipc/loop.ts';
import { kind, type Outgoing } from '../ipc/protocol.ts';

/** 启动参数(与 Go 侧 service/proxy.StartRequest 对齐)。 */
interface StartRequest {
  proxyId: string;
  name: string;
  host: string;
  port: number;
  uiport: number;
  timeout?: number;
  dnsServer?: string;
  basedir?: string;
}

interface RunningProxy {
  /** starting 的实例键(含 #proxyId), 兜底模式下为空 */
  key?: string;
  /** 兜底模式下的 wrapper 进程 */
  child?: ChildProcess;
  port: number;
  uiport: number;
}

const instances = new Map<string, RunningProxy>();
const requireFromHere = createRequire(import.meta.url);

/** whistle 的安装目录(dist/vendor 或 node_modules)。 */
function resolveWhistleRoot(): string | null {
  try {
    const vendored = fileURLToPath(new URL('./vendor/whistle/package.json', import.meta.url));
    if (existsSync(vendored)) return path.dirname(vendored);
  } catch {
    // 非 ESM 环境
  }
  try {
    return path.dirname(requireFromHere.resolve('whistle/package.json'));
  } catch {
    return null;
  }
}

/** whistle CLI 入口(兜底模式用)。 */
function resolveWhistleBin(): string {
  const fromEnv = process.env.WHISTLE_BIN;
  if (fromEnv && existsSync(fromEnv)) return fromEnv;
  try {
    const vendored = fileURLToPath(new URL('./vendor/whistle/bin/whistle.js', import.meta.url));
    if (existsSync(vendored)) return vendored;
  } catch {
    // 非 ESM 环境
  }
  try {
    return requireFromHere.resolve('whistle/bin/whistle.js');
  } catch (err) {
    throw new Error(`whistle not resolvable (set WHISTLE_BIN, run pnpm build, or install it): ${String(err)}`);
  }
}

/** starting 库(whistle 的进程管理) —— 不可用时返回 null, 由调用方走兜底。 */
function loadStarting(root: string | null): { cli: any } | null {
  if (!root) return null;
  try {
    const req = createRequire(path.join(root, 'package.json'));
    const starting = req('starting');
    return starting?.cli ? starting : null;
  } catch {
    return null;
  }
}

/**
 * 确保工作目录存在。
 * spawn 的 cwd 不存在时, Node 报的是 **ENOENT**(看起来像可执行文件找不到), 很容易误导。
 */
function ensureDir(target: string): string {
  try {
    mkdirSync(target, { recursive: true });
    return target;
  } catch {
    return process.cwd();
  }
}

/** 极简 TCP 探测(不引入额外依赖)。 */
function netConnect(port: number): import('node:net').Socket {
  const net = requireFromHere('node:net') as typeof import('node:net');
  const socket = new net.Socket();
  socket.setTimeout(500);
  socket.connect(port, '127.0.0.1');
  return socket;
}

/** 等待端口可连(就绪信号), 超时或子进程提前退出都视为失败。 */
function waitReady(port: number, timeoutMs: number, child?: ChildProcess): Promise<void> {
  return new Promise((resolve, reject) => {
    const deadline = setTimeout(() => {
      cleanup();
      reject(new Error(`whistle did not listen on ${port} within ${timeoutMs}ms`));
    }, timeoutMs);
    const onExit = (code: number | null) => {
      cleanup();
      reject(new Error(`whistle exited early (code=${code})`));
    };
    const cleanup = () => {
      clearTimeout(deadline);
      if (child) child.off('exit', onExit);
    };
    if (child) child.once('exit', onExit);

    const probe = () => {
      const socket = netConnect(port);
      socket.once('connect', () => {
        socket.destroy();
        cleanup();
        resolve();
      });
      socket.once('error', () => {
        socket.destroy();
        if (!child || !child.killed) setTimeout(probe, 120);
      });
    };
    probe();
  });
}

/**
 * 按端口反查 pid: whistle 会把真正监听的进程 detach 出去(与 wrapper 不同 pid、不同进程组),
 * 也不写 pid 文件 —— 只能从监听表里找。Linux 用 ss, macOS 用 lsof。
 */
function pidsByPort(port: number): number[] {
  const childProcess = requireFromHere('node:child_process') as typeof import('node:child_process');
  const run = (command: string, args: string[]): string => {
    try {
      return String(
        childProcess.execFileSync(command, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }),
      );
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

/** 停掉真正监听该端口的进程: TERM -> 等 -> KILL; 返回是否清理干净。 */
async function stopByPort(port: number, timeoutMs: number): Promise<boolean> {
  for (const pid of pidsByPort(port)) {
    try {
      process.kill(pid, 'SIGTERM');
    } catch {
      // 已退出
    }
  }
  const deadline = Date.now() + Math.max(1000, Math.floor(timeoutMs / 2));
  while (Date.now() < deadline) {
    if (pidsByPort(port).length === 0) return true;
    await new Promise((resolve) => setTimeout(resolve, 150));
  }
  for (const pid of pidsByPort(port)) {
    try {
      process.kill(pid, 'SIGKILL');
    } catch {
      // 已退出
    }
  }
  await new Promise((resolve) => setTimeout(resolve, 200));
  return pidsByPort(port).length === 0;
}

/** 向整个进程组发信号(兜底模式下 whistle 的 wrapper 会 fork 出真正的代理进程)。 */
function signalTree(child: ChildProcess, signal: NodeJS.Signals): void {
  const pid = child.pid;
  if (!pid) return;
  try {
    if (process.platform !== 'win32') {
      process.kill(-pid, signal);
      return;
    }
  } catch {
    // 组不存在(已退出)时退回单进程
  }
  try {
    child.kill(signal);
  } catch {
    // 已退出
  }
}

export const proxyHandlers: Record<string, Handler> = {};

proxyHandlers['proxy.start'] = async (packet): Promise<Outgoing> => {
  const req = decodeData<StartRequest>(packet);
  let startedPid = 0;
  const root = resolveWhistleRoot();
  const starting = loadStarting(root);
  const cwd = ensureDir(req.basedir || process.cwd());

  if (starting && root) {
    // 官方 standalone: starting 按配置启动并记录 pid。
    // 实例隔离键的格式是 `<main>#<id>#` —— starting 只在**以 # 结尾**时才识别并剥离
    // (见 starting/lib/cli.js 的 getHashIndex), 漏掉结尾的 # 会把整串当模块路径。
    const key = `${path.join(root, 'index.js')}#${req.proxyId}#`;
    const options: Record<string, unknown> = {
      host: req.host,
      port: req.port,
      uiport: req.uiport,
      baseDir: cwd,
    };
    if (req.timeout) options.timeout = req.timeout;
    if (req.dnsServer) options.dnsServer = req.dnsServer;

    await new Promise<void>((resolve, reject) => {
      starting.cli.start(key, options, (err: unknown) => {
        if (err) reject(new Error(`starting.cli.start failed: ${String(err)}`));
        else resolve();
      });
    });
    await waitReady(req.uiport, 15000);
    instances.set(req.proxyId, { key, port: req.port, uiport: req.uiport });
    // starting 会把实例 pid 写入配置文件, 这里读出来作为真实 pid 回报
    try {
      const record = starting.cli.readConfig?.(key);
      if (record?.pid) startedPid = Number(record.pid);
    } catch {
      // 读不到就用 0(仅展示用)
    }
  } else {
    // 兜底: 直接跑 whistle CLI
    const bin = resolveWhistleBin();
    const args = [
      'run',
      '-D', cwd,
      '-p', String(req.port),
      '-P', String(req.uiport),
    ];
    if (req.host) args.push('-H', req.host);
    if (req.timeout) args.push('-t', String(req.timeout));
    if (req.dnsServer) args.push('--dnsServer', req.dnsServer);

    const child = spawn(resolveNodeBin(), [bin, ...args], {
      cwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env },
      detached: process.platform !== 'win32',
    });
    child.stdout?.on('data', (chunk) => console.log(`[whistle:${req.name}] ${String(chunk).trim()}`));
    child.stderr?.on('data', (chunk) => console.error(`[whistle:${req.name}] ${String(chunk).trim()}`));

    const failed = new Promise<never>((_, reject) => {
      child.once('error', (err) => reject(new Error(`spawn whistle failed (cwd=${cwd}): ${String(err)}`)));
    });
    await Promise.race([waitReady(req.uiport, 15000, child), failed]);
    instances.set(req.proxyId, { child, port: req.port, uiport: req.uiport });
  }

  return {
    id: packet.id,
    kind: kind.stream,
    name: 'proxy.started',
    data: {
      proxyId: req.proxyId,
      pid: startedPid,
      port: req.port,
      uiport: req.uiport,
      url: `http://${req.host}:${req.uiport}`,
    },
  };
};

proxyHandlers['proxy.stop'] = async (packet): Promise<Outgoing> => {
  const payload = decodeData<{ proxyId: string; timeoutMs?: number }>(packet);
  const running = instances.get(payload.proxyId);
  const timeoutMs = payload.timeoutMs ?? 5000;

  if (running?.key) {
    const root = resolveWhistleRoot();
    const starting = loadStarting(root);
    if (starting) {
      await new Promise<void>((resolve) => {
        starting.cli.stop(running.key, () => resolve());
      });
    }
  }

  if (running?.child && !running.child.killed) {
    signalTree(running.child, 'SIGTERM');
    await new Promise((resolve) => setTimeout(resolve, Math.max(500, Math.floor(timeoutMs / 3))));
    if (!running.child.killed) signalTree(running.child, 'SIGKILL');
  }

  // 最终校验: 端口必须释放(两种模式都适用; starting 停止失败时这里兜底)
  if (running) {
    const uiportFree = await stopByPort(running.uiport, timeoutMs);
    const portFree = running.uiport === running.port ? true : await stopByPort(running.port, timeoutMs);
    if (!uiportFree || !portFree) {
      console.error(
        `[proxy.stop] 端口未释放 proxy=${payload.proxyId} port=${running.port} uiport=${running.uiport}`,
      );
    }
    instances.delete(payload.proxyId);
  }

  return {
    id: packet.id,
    kind: kind.stream,
    name: 'proxy.stopped',
    data: { proxyId: payload.proxyId, at: Date.now() },
  };
};

/** node 可执行: 优先当前进程(存在才算), 否则交给 PATH。 */
function resolveNodeBin(): string {
  const candidates = [process.execPath, process.env.NODE_BIN, process.env.npm_node_execpath];
  for (const candidate of candidates) {
    if (candidate && existsSync(candidate)) return candidate;
  }
  return 'node';
}
