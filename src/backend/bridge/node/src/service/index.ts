import type { Handler } from '../ipc/loop.ts';
import { healthHandlers } from './health.ts';
import { proxyHandlers } from './proxy.ts';

/**
 * 业务处理器总表: 每个服务一个独立源文件(health / proxy / ...),
 * 这里只做汇总, 不承载任何具体处理逻辑。
 */
export const handlers: Record<string, Handler> = {
  ...healthHandlers,
  ...proxyHandlers,
};
