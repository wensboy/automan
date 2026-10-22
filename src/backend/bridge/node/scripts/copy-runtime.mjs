// 把 **必须按路径解析** 的运行时依赖拷进 dist, 使落盘后的子进程仍能 require 到。
//
// 目前只有 pfork(约 40KB, 无依赖): 它内部用 __dirname 定位 lib/main, 因此不能被打包成
// bundle, 只能以"整包随产物"的形式存在。whistle 体积大(13MB), 仍按 npm 依赖解析
// (开发期由 NODE_PATH 指向 node_modules; 分发期需要能解析到 whistle, 见 v23 §21)。
import { cpSync, existsSync, mkdirSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';

const requireFromHere = createRequire(import.meta.url);

const copyPackage = (name, targetDir) => {
  let source;
  try {
    source = path.dirname(requireFromHere.resolve(`${name}/package.json`));
  } catch (err) {
    console.error(`[runtime] 无法解析 ${name}: ${err.message}`);
    process.exit(1);
  }
  mkdirSync(targetDir, { recursive: true });
  // 覆盖式拷贝(不做整目录删除: 某些环境对批量删除有守卫)
  cpSync(source, path.join(targetDir, name), { recursive: true, force: true });
  const version = JSON.parse(readFileSync(path.join(source, 'package.json'), 'utf8')).version;
  console.log(`[runtime] ${name}@${version} -> ${path.relative(process.cwd(), path.join(targetDir, name))}`);
};

const dist = path.join(process.cwd(), 'dist');
if (!existsSync(dist)) {
  console.error('[runtime] dist 不存在: 先执行 esbuild 构建');
  process.exit(1);
}
copyPackage('pfork', path.join(dist, 'node_modules'));
