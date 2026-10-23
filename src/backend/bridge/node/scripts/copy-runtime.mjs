// 把"运行期必须按路径解析"的依赖拷进 dist, 使落盘后的子进程仍能 require 到。
//
// 说明(见 docs/design/v23.md §34):
//   · pfork: 内部用 __dirname 定位 lib/main, 不能 bundle -> 整包拷贝;
//   · whistle: 同样按 __dirname 动态 require 自己的 lib、读取 assets, 不能 bundle;
//     这里把 whistle **连同其自带 node_modules** 一起拷, 其余生产依赖平铺到 dist/node_modules,
//     用二进制体积换"开箱即用"(实测约 13MB)。
//   · 依赖解析必须**从各自包目录**出发: pnpm 布局下依赖不一定提升到顶层。
//   · 不做整目录删除(某些环境对批量删除有守卫); 覆盖式写入即可。
import { cpSync, existsSync, mkdirSync, readFileSync, realpathSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';

const requireHere = createRequire(import.meta.url);
const dist = path.join(process.cwd(), 'dist');
const targetRoot = path.join(dist, 'node_modules');

if (!existsSync(dist)) {
  console.error('[runtime] dist 不存在: 先执行 esbuild 构建');
  process.exit(1);
}
mkdirSync(targetRoot, { recursive: true });

const readPkg = (dir) => JSON.parse(readFileSync(path.join(dir, 'package.json'), 'utf8'));

/** 从 fromDir 出发解析包目录(处理 pnpm 的软链布局)。 */
const resolvePackageDir = (name, fromDir) => {
  try {
    const base = fromDir ? path.join(fromDir, 'package.json') : import.meta.url;
    return realpathSync(path.dirname(createRequire(base).resolve(`${name}/package.json`)));
  } catch {
    return '';
  }
};

const copied = new Map();

const copyPackage = (name, fromDir, withNested) => {
  if (copied.has(name)) return copied.get(name);
  const source = resolvePackageDir(name, fromDir) || resolvePackageDir(name, '');
  if (!source) {
    copied.set(name, '');
    console.warn(`[runtime] 跳过(无法解析): ${name}`);
    return '';
  }
  const target = path.join(targetRoot, name);
  if (!existsSync(target)) {
    mkdirSync(path.dirname(target), { recursive: true });
    cpSync(source, target, {
      recursive: true,
      force: true,
      // whistle 需要自带的 node_modules(其内部按相对路径 require); 其它包只拷源码
      filter: withNested ? undefined : (src) => path.basename(src) !== 'node_modules',
    });
    const version = readPkg(source).version ?? '0.0.0';
    console.log(`[runtime] ${name}@${version} -> dist/node_modules/${name}`);
  }
  copied.set(name, source);
  return source;
};

/** 广度优先收集生产依赖闭包(devDependencies 不参与运行时)。 */
const collect = (roots) => {
  const queue = roots.map((name) => ({ name, from: '', nested: name === 'whistle' }));
  const seen = new Set();
  while (queue.length > 0) {
    const item = queue.shift();
    if (!item || seen.has(item.name)) continue;
    seen.add(item.name);
    const source = copyPackage(item.name, item.from, item.nested);
    if (!source) continue;
    const deps = Object.keys(readPkg(source).dependencies ?? {});
    for (const dep of deps) queue.push({ name: dep, from: source, nested: false });
  }
};

collect(['pfork', 'whistle']);

const size = (dir) => {
  let total = 0;
  for (const entry of requireHere('node:fs').readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    try {
      total += entry.isDirectory() ? size(full) : requireHere('node:fs').statSync(full).size;
    } catch {
      // 悬空软链等忽略
    }
  }
  return total;
};

console.log(
  `[runtime] 完成: dist/node_modules 共 ${copied.size} 个包, ${(size(targetRoot) / 1024 / 1024).toFixed(1)}MB`,
);
