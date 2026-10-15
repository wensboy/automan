import type { Frame } from './protocol.ts';

/** 帧标准: {"id":"...","payload":"<base64>"} —— 见 Go 侧 ipc/frame.go。 */
export function encodeFrame(id: string, payload: Uint8Array): string {
  const frame: Frame = { id, payload: Buffer.from(payload).toString('base64') };
  return JSON.stringify(frame);
}

export function decodeFrame(text: string): { id: string; payload: Uint8Array } {
  const frame = JSON.parse(text) as Frame;
  return {
    id: frame.id,
    payload: frame.payload ? Buffer.from(frame.payload, 'base64') : new Uint8Array(),
  };
}
