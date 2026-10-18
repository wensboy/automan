// 把 whistle 及其**生产依赖**按 flat node_modules 布局拷进 dist/vendor,
// 使 dist 自洽(可被 Go 内嵌后独立运行), 同时不破坏开发期用 WHISTLE_BIN 的灵活性。
//
// 为什么不做 bundle: whistle 用 __dirname 动态 require 自己的 lib(插件/服务按路径加载),
// 单文件打包后 __dirname 变成产物目录 -> 运行即失败(实测 Cannot find module '<dir>/lib/server')。
import { createRequire } from 'node:module';
import { cpSync, existsSync, mkdirSync, readFileSync, readdirSync, realpathSync, statSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const nodeDir = path.resolve(scriptDir, '..');
const vendorDir = path.join(nodeDir, 'dist', 'vendor');

const packageRoot = (name, fromDir) => {
  const req = createRequire(path.join(fromDir, 'package.json'));
  try {
    return realpathSync(path.dirname(req.resolve(`${name}/package.json`)));
  } catch {
    // 一些包不导出 package.json: 从入口文件向上找最近的 package.json
    try {
      let dir = realpathSync(path.dirname(req.resolve(name)));
      while (dir !== path.dirname(dir)) {
        if (existsSync(path.join(dir, 'package.json'))) return dir;
        dir = path.dirname(dir);
      }
    } catch {
      // 解析不到: 交由调用方决定(可选依赖允许缺失)
    }
    return null;
  }
};

const readPkg = (dir) => JSON.parse(readFileSync(path.join(dir, 'package.json'), 'utf8'));

const whistleDir = packageRoot('whistle', nodeDir);
if (!whistleDir) {
  console.error('[vendor] whistle 未安装, 跳过(开发期请 pnpm add whistle)');
  process.exit(0);
}

// 收集传递依赖(只走 dependencies; optionalDependencies 缺失不算错)
const collected = new Map(); // name -> dir
const queue = [whistleDir];
const seen = new Set([whistleDir]);
while (queue.length > 0) {
  const dir = queue.shift();
  const pkg = readPkg(dir);
  for (const dep of Object.keys(pkg.dependencies ?? {})) {
    const depDir = packageRoot(dep, dir);
    if (!depDir) {
      console.warn(`[vendor] 跳过无法解析的依赖: ${dep} (from ${pkg.name})`);
      continue;
    }
    if (collected.has(dep) || seen.has(depDir)) continue;
    collected.set(dep, depDir);
    seen.add(depDir);
    queue.push(depDir);
  }
}

// 不做整目录删除: 某些环境对"批量删除"有守卫(本机 VS Code 的 safe-delete 会直接抛错),
// 且删除对增量构建没有收益 —— 直接覆盖同名文件即可(版本变化时写入标记, 便于排查)。
mkdirSync(path.join(vendorDir, 'node_modules'), { recursive: true });

// whistle 本体: dist/vendor/whistle(带上它自带的嵌套 node_modules)
cpSync(whistleDir, path.join(vendorDir, 'whistle'), {
  recursive: true,
  dereference: true,
  filter: (src) => !src.endsWith('.map'),
});

// 依赖: flat 布局 dist/vendor/node_modules/<name>(跳过各自的 node_modules, 由扁平层解析)
for (const [name, dir] of collected) {
  const target = path.join(vendorDir, 'node_modules', name);
  mkdirSync(path.dirname(target), { recursive: true });
  cpSync(dir, target, {
    recursive: true,
    dereference: true,
    filter: (src) => {
      const rel = path.relative(dir, src);
      if (rel.split(path.sep).includes('node_modules')) return false;
      return !src.endsWith('.map');
    },
  });
}

const walkSize = (dir) => {
  let total = 0;
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    try {
      if (entry.isDirectory()) total += walkSize(full);
      else if (entry.isFile()) total += statSync(full).size;
    } catch {
      // 悬空软链等: 忽略(不计入体积)
    }
  }
  return total;
};

writeFileSync(
  path.join(vendorDir, '.whistle-version'),
  `${readPkg(whistleDir).version}\n`,
);

const size = walkSize(vendorDir);
console.log(
  `[vendor] whistle + ${collected.size} 个依赖 -> dist/vendor, 合计 ${(size / 1024 / 1024).toFixed(1)} MB`,
);
