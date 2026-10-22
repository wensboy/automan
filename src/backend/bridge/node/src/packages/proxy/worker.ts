/**
 * pfork 入口: 在**子进程**里以 npm 模块方式启动 whistle。
 *
 * 关键事实(实测, 见 docs/design/v23.md §22):
 *   · `require('whistle')` 的导出**是函数**: whistle(options, cb) 就地启动服务;
 *   · 它的回调首参**不保证是 error**(成功时也可能给一个对象), 因此不能 `if (err) 失败`;
 *     真正的成功判据是"端口已经 LISTEN"。
 *   · whistle 内部还会 fork 服务进程(detached), 停止由 manager 用 pfork 按 storage 追踪。
 *
 * 本文件保持 CommonJS 导出函数形态: 这是 pfork 加载脚本的约定(`module.exports = fn`)。
 */
/* eslint-disable @typescript-eslint/no-var-requires */
declare const require: (id: string) => any;
declare const module: { exports: unknown };
declare const process: NodeJS.Process;

const net = require('node:net');
const pfork = require('pfork');

type WhistleFunction = (
  options: Record<string, unknown>,
  cb: (first?: unknown, second?: unknown) => void,
) => void;

// whistle 体积大, 不随产物内嵌: 解析不到时给出明确指引(而不是让 pfork 报一个笼统失败)
const whistle = ((): WhistleFunction => {
  try {
    return require('whistle');
  } catch (err) {
    throw new Error(
      'whistle 不可解析: 在开发期由 NODE_PATH 指向 node_modules(见 conf/bridge.json), ' +
        '分发环境需保证 whistle 可被 node 解析。原始错误: ' + String(err),
    );
  }
})();

/** pfork 传给脚本的 options: 见 manager.start()。 */
interface WorkerOptions {
  storage: string;
  host: string;
  port: number;
  uiport: number;
  timeout?: number;
  dnsServer?: string;
}

/** 单次 TCP 探测: 目标端口是否能连上。 */
function probe(host: string, port: number, timeoutMs = 500): Promise<boolean> {
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

/** 等到端口真正 LISTEN(whistle 的回调时机不可靠, 以监听状态为准)。 */
async function waitListening(host: string, port: number, timeoutMs: number): Promise<boolean> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await probe(host, port)) return true;
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  return false;
}

module.exports = function worker(options: WorkerOptions, callback: (err: unknown, data?: unknown) => void): void {
  const startedAt = Date.now();
  const waitMs = options.timeout ?? 30000;
  let settled = false;

  const settle = (err: unknown, data?: unknown): void => {
    if (settled) return;
    settled = true;
    callback(err, data);
  };

  whistle(
    {
      host: options.host,
      port: options.port,
      uiport: options.uiport,
      baseDir: options.storage,
      timeout: waitMs,
      dnsServer: options.dnsServer ?? '',
    },
    () => {
      // 首参可能是 options 也可能是 error: 不看它, 统一以端口探测为准
      void (async () => {
        const listening = await waitListening(options.host, options.uiport, waitMs);
        if (!listening) {
          settle(new Error(`whistle 未在 ${options.host}:${options.uiport} 监听(等待 ${waitMs}ms)`));
          return;
        }
        await waitListening(options.host, options.port, 3000);
        settle(null, {
          pid: process.pid,
          port: options.port,
          uiport: options.uiport,
          elapsedMs: Date.now() - startedAt,
        });
      })();
    },
  );

  // 启动过程中异常退出: 明确回报(否则 pfork 侧只会看到一个笼统失败)
  process.on('uncaughtException', (err: Error) => {
    settle(new Error(`whistle 启动异常: ${err.message}`));
  });

  // 结束本进程前, 让 whistle 自己的服务进程也退出(按 storage 追踪)。
  process.on('SIGTERM', () => {
    try {
      const requireResolve = (require as unknown as { resolve(id: string): string }).resolve;
      pfork.kill({ script: requireResolve('whistle/lib/service/service.js'), value: options.storage });
    } catch {
      // 追踪不到时不阻塞退出(manager 会做端口兜底)
    }
    process.exit(0);
  });
};
