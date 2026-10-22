import type { Peer } from './client.ts';
import { name, protocolVersion } from './protocol.ts';

export interface HandshakeOptions {
  daemon: string;
  codec: string;
  token: string;
}

interface HandshakeResult {
  daemon: string;
  codec: string;
  version: number;
}

/**
 * 客户端握手: 用 **request/response** 语义(v3), 失败即抛错。
 * 版本/编码不匹配时父进程会回 error 帧, 这里原样上抛。
 */
export async function handshake(peer: Peer, options: HandshakeOptions): Promise<HandshakeResult> {
  const env = await peer.request(
    name.handshake,
    { daemon: options.daemon, codec: options.codec, version: protocolVersion, token: options.token },
    5000,
  );
  const result = env.data as HandshakeResult | undefined;
  if (!result || result.version !== protocolVersion) {
    throw new Error(`version mismatch: want ${protocolVersion}, got ${result?.version}`);
  }
  if (result.codec !== options.codec) {
    throw new Error(`codec mismatch: want ${options.codec}, got ${result.codec}`);
  }
  return result;
}
