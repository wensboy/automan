import { build, context, type BuildOptions } from 'esbuild';

// 两个入口: index(守护进程主体) + worker(pfork 启动 whistle 的脚本)。
// worker 必须是独立文件: pfork 以 `node <script>` 方式加载, 不能内联到单文件里。
// 用 .cts(CJS) 而不是 .ts: 包是 "type": "module", 在 .ts 里写 module.exports 会被
// esbuild 判定为 ESM 中的 CommonJS 变量(commonjs-variable-in-esm 警告), .cts 明确是 CJS。
const common: BuildOptions = {
  bundle: true,
  platform: 'node',
  target: 'node20',
  sourcemap: true,
  // whistle / pfork 作为运行时依赖(由 npm 安装或随包分发), 不打进 bundle。
  external: ['whistle', 'pfork', 'bufferutil', 'utf-8-validate'],
  logLevel: 'info',
};

const options: BuildOptions[] = [
  {
    ...common,
    entryPoints: ['src/index.ts'],
    outfile: 'dist/index.js',
    format: 'esm',
    // esm 产物中为 CJS 依赖提供 require。
    banner: {
      js: "import { createRequire as __createRequire } from 'node:module'; const require = __createRequire(import.meta.url);",
    },
  },
  {
    ...common,
    entryPoints: ['src/packages/proxy/worker.cts'],
    outfile: 'dist/worker.cjs',
    format: 'cjs',
  },
];

async function main(): Promise<void> {
  if (!process.argv.includes('--watch')) {
    await Promise.all(options.map((option) => build(option)));
    return;
  }
  const contexts = await Promise.all(options.map((option) => context(option)));
  await Promise.all(contexts.map((ctx) => ctx.watch()));
}

main().catch((err: unknown) => {
  console.error(err);
  process.exit(1);
});
