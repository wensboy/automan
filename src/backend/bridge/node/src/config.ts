import { parseArgs } from 'node:util';

export interface Config {
  sock: string;
  daemon: string;
  codec: string;
  token: string;
  reconnectBaseMs: number;
  reconnectMaxMs: number;
  reconnectAttempts: number;
  livenessMs: number;
}

/** 解析启动参数: argv 优先, env 兜底 (sock 由父进程注入)。 */
export function parseConfig(argv: string[]): Config {
  const { values } = parseArgs({
    args: argv,
    options: {
      sock: { type: 'string' },
      daemon: { type: 'string' },
      codec: { type: 'string' },
      token: { type: 'string' },
      'reconnect-base': { type: 'string' },
      'reconnect-max': { type: 'string' },
      'reconnect-attempts': { type: 'string' },
      'liveness-ms': { type: 'string' },
    },
  });

  const sock = values.sock ?? process.env['AUTOMAN_BRIDGE_SOCK'] ?? '';
  if (sock === '') {
    throw new Error('missing sock: pass --sock=<path> or set AUTOMAN_BRIDGE_SOCK');
  }

  return {
    sock,
    daemon: values.daemon ?? 'node',
    codec: values.codec ?? 'json',
    token: values.token ?? '',
    reconnectBaseMs: toInt(values['reconnect-base'], 200),
    reconnectMaxMs: toInt(values['reconnect-max'], 5000),
    reconnectAttempts: toInt(values['reconnect-attempts'], 0),
    livenessMs: toInt(values['liveness-ms'], 15000),
  };
}

function toInt(raw: string | undefined, fallback: number): number {
  if (raw === undefined) return fallback;
  const value = Number.parseInt(raw, 10);
  return Number.isFinite(value) ? value : fallback;
}
