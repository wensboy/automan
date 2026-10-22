/**
 * 代理服务: 只做协议适配, 生命周期交给 packages/proxy 的无状态函数。
 *
 * 设计(docs/design/v23.md §20):
 *   · 每个代理一个 whistle 实例, 以 **storage(=basedir)** 作为唯一标识;
 *   · 启动/停止都由 pfork 按 script+storage 追踪, 服务层不维护进程表(无状态);
 *   · whistle 以 npm 模块方式在 worker 进程内启动(官方 standalone 用法)。
 */
import path from 'node:path';

import { decodeData } from '../ipc/envelope.ts';
import type { Handler } from '../ipc/loop.ts';
import { kind, type Outgoing } from '../ipc/protocol.ts';
import { start, stop } from '../packages/proxy/manager.ts';
import { messageOf } from '../packages/share/index.ts';

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

interface StopRequest {
  proxyId: string;
  timeoutMs?: number;
  /** 由 Go 侧按代理名解析出的 basedir(= storage)。 */
  dir?: string;
}

/** storage 是实例的唯一标识: 优先用 Go 侧给的 basedir。 */
function storageOf(proxyId: string, basedir?: string): string {
  if (basedir && basedir.trim() !== '') return basedir;
  return path.join(process.cwd(), 'proxies', proxyId);
}

export const proxyHandlers: Record<string, Handler> = {};

proxyHandlers['proxy.start'] = async (packet): Promise<Outgoing> => {
  const req = decodeData<StartRequest>(packet);
  const storage = storageOf(req.proxyId, req.basedir);

  const started = await start({
    storage,
    host: req.host,
    port: req.port,
    uiport: req.uiport,
    timeout: req.timeout,
    dnsServer: req.dnsServer,
  });

  return {
    id: packet.id,
    kind: kind.stream,
    name: 'proxy.started',
    data: {
      proxyId: req.proxyId,
      pid: started.pid,
      port: started.port,
      uiport: started.uiport,
      url: `http://${req.host}:${started.uiport}`,
    },
  };
};

proxyHandlers['proxy.stop'] = async (packet): Promise<Outgoing> => {
  const req = decodeData<StopRequest>(packet);
  const storage = storageOf(req.proxyId, req.dir);
  try {
    await stop(storage, req.timeoutMs ?? 5000);
  } catch (err) {
    // 停止失败必须显式抛出: 由 loop 回 error 帧, 避免"假成功"
    throw new Error(`proxy.stop failed (${storage}): ${messageOf(err)}`);
  }
  return {
    id: packet.id,
    kind: kind.stream,
    name: 'proxy.stopped',
    data: { proxyId: req.proxyId, at: Date.now() },
  };
};
