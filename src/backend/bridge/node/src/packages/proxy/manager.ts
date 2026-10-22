/**
 * 无状态生命周期函数: 全部通过 **pfork + storage** 定位实例(不维护进程表)。
 *
 *   start(options)  启动一个 whistle 实例(同一 storage 重复调用会被 pfork 去重)
 *   stop(storage)   结束该 storage 对应的实例
 *   exists(storage) 是否仍在运行
 */
/* eslint-disable @typescript-eslint/no-var-requires */
declare const require: (id: string) => any;

import { killByPort, messageOf, portListening, sleep } from '../share/index.ts';

const pfork = require('pfork');

/** worker 脚本路径: 构建后与本文件同目录(dist)。 */
const workerScript = ((): string => {
  // 运行期以落地目录为准(dist/index.js -> dist/worker.js); 开发期 tsx 下也成立
  const url = new URL('./worker.cjs', import.meta.url);
  return decodeURIComponent(url.pathname);
})();

export interface StartOptions {
  /** 实例标识(用于 pfork 去重/追踪, 同时作为 whistle 的 baseDir)。 */
  storage: string;
  host: string;
  port: number;
  uiport: number;
  timeout?: number | undefined;
  dnsServer?: string | undefined;
}

export interface Started {
  pid: number;
  port: number;
  uiport: number;
}

/** 启动: 已存在则直接返回(幂等)。 */
export function start(options: StartOptions): Promise<Started> {
  return new Promise((resolve, reject) => {
    pfork.fork(
      {
        script: workerScript,
        value: options.storage,
        ...options,
      },
      (err: unknown, data: Partial<Started> | undefined) => {
        if (err) {
          reject(new Error(`proxy start failed: ${messageOf(err)}`));
          return;
        }
        const started: Started = {
          pid: Number(data?.pid ?? 0),
          port: Number(data?.port ?? options.port),
          uiport: Number(data?.uiport ?? options.uiport),
        };
        // 记录端口, 让 stop 仅凭 storage 就能定位并校验实例(设计的"无状态"前提)
        try {
          const fs = require('node:fs') as typeof import('node:fs');
          fs.mkdirSync(options.storage, { recursive: true });
          fs.writeFileSync(`${options.storage}/.bridge-ports.json`, JSON.stringify(started));
        } catch {
          // 记录失败不影响启动
        }
        resolve(started);
      },
    );
  });
}

/**
 * 结束: 先按 storage 让 pfork 收掉 worker, 再按端口确认真正监听的进程已退出。
 *
 * 为什么不能只靠 pfork.kill: whistle 启动时把它自己的服务进程 detach 出去(与 worker 不同 pid/进程组),
 * 只杀 worker 端口不会释放(实测)。因此这里以"端口是否释放"为最终判据, 不释放就抛错(不假成功)。
 */
export async function stop(storage: string, timeoutMs = 5000): Promise<void> {
  try {
    // pfork.kill 没有回调(README 即如此), 不能等它; 失败也可能只是"本来就没在跑"
    pfork.kill({ script: workerScript, value: storage });
  } catch {
    // 忽略: 下面按端口判定真实状态
  }
  await sleep(300);

  const ports: number[] = [];
  try {
    const info = JSON.parse(String(require('node:fs').readFileSync(`${storage}/.bridge-ports.json`, 'utf8'))) as {
      port?: number;
      uiport?: number;
    };
    if (info.uiport) ports.push(info.uiport);
    if (info.port) ports.push(info.port);
  } catch {
    // 没有记录时只做一次"按 storage 名"的兜底: 调用方应传端口(见 service 层)
  }

  for (const port of ports) {
    await killByPort(port, timeoutMs);
  }
  for (const port of ports) {
    if (await portListening('127.0.0.1', port)) {
      throw new Error(`端口未释放: ${port}`);
    }
  }
}

/** 是否仍在运行(按 storage 追踪)。 */
export function exists(storage: string): boolean {
  try {
    return Boolean(pfork.exists({ script: workerScript, value: storage }));
  } catch {
    return false;
  }
}
