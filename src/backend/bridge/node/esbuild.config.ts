import { build, context, type BuildOptions } from 'esbuild';

// 打包为单文件产物: 依赖内联 (含 ws), 便于后续嵌入 Go 二进制后独立运行。
const options: BuildOptions = {
  entryPoints: ['src/index.ts'],
  outfile: 'dist/index.js',
  bundle: true,
  platform: 'node',
  target: 'node20',
  format: 'esm',
  sourcemap: true,
  // ws 的可选原生依赖: 不在本地, 运行期由 ws 自行 try/catch 兜底。
  external: ['whistle', 'bufferutil', 'utf-8-validate'],
  // esm 产物中为 CJS 依赖提供 require。
  banner: {
    js: "import { createRequire as __createRequire } from 'node:module'; const require = __createRequire(import.meta.url);",
  },
  logLevel: 'info',
};

async function main(): Promise<void> {
  if (!process.argv.includes('--watch')) {
    await build(options);
    return;
  }
  const ctx = await context(options);
  await ctx.watch();
}

main().catch((err: unknown) => {
  console.error(err);
  process.exit(1);
});
