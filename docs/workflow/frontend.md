---
name: "frontend workflow"
description: "frontend 必要的规范"
---

## 规范

1. 所有静态数据在 `frontend/src/static/` 下, 以高度语义化的形式存储为json对象.
2. 所有组件定义到 `frontend/src/components/custom/` 下; 共享组件定义到 `frontend/src/components/custom/share/` 下.
3. 页面定义到 `frontend/src/pages/` 下; 布局定义到 `frontend/src/layouts/` 下.
4. 在完善 ui 时总是需要考虑将 mock 数据定义到 `frontend/src/mock/` 下, 对接完成后测试并移除 mock.
5. 总是优先使用 shadcn/ui 原生组件. 考虑最大 3 轮无法达到效果则尝试询问用户是否使用自定义组件处理, 不做过多无效尝试.
6. 保持界面风格和类似组件风格显示始终一致.
7. 页面静态资源在 `frontend/public/`, 通常包含图像, 字体, svg等.
8. 组件遵循内聚和复用原则, 界定有状态无状态来实现复用和组合.

## 细节

1. 静态数据通常包含: ui组件静态文本, ui组件动态文本映射, 组件显示定义, 组件静态类型定义, 灵活拓展覆盖.
2. lib/*.ts 遵循单一文件单一职责原则, 允许多组件模块共享使用.
3. 静态层覆盖所有可见文案与 `aria-label/placeholder/title/alt`(`components/ui/*` 的 shadcn 默认文案除外).
4. 共享组件禁止依赖 mock: 展示类型随组件或 lib 定义; mock 仅服务未对接页面, 对接后删除并将类型/格式化迁入 lib.
5. 图标名以 kebab-case 存在静态层, 映射只在 `lib/icons.ts` 注册, 组件统一用 `resolveIcon` 使用.
6. 图表容器必须有确定尺寸: 用 flex 定高链或固定像素高度; flex 居中容器内不要用百分比高度(会算成 0).
7. 通知/抽屉长文本: 标题与内容默认都**单行截断**(`truncate`), 点击条目(或展开按钮)展示全文, 展开态用 `break-all whitespace-pre-wrap` 防长路径横向溢出; toast 的 description 保持 `line-clamp-2`; toast 左侧 icon 与标题首行对齐(`items-start`).
8. 编辑保存交互: 脏值时在文件名右侧显示保存按钮, 保存成功后消失; 展示用 pretty JSON, 存储可压缩.
9. 服务端错误通过 dialog 内错误文本或 `notify` 呈现, 文案必须来自静态层.
10. 需求/决策历史留在 vxx.md; 前端通用规范写入本文件 `## 细节`.
11. 工具链约束: `node >= 24`, `pnpm >= 11`; 由 `frontend/package.json` 的 `engines` + `.npmrc` 的 `engine-strict=true` 强制, 不满足直接安装失败.
12. 滚动条统一策略: 所有位置始终隐藏滚动条(零宽不占位), 只用滚轮/触控板滚动; 实现只在 `src/index.css` 的 `@layer base`, 组件不要各自覆写. 第三方自绘滚动条(xterm、Radix `ScrollArea`)同样收起.
13. 关闭动画必须保持末帧: `animate-out` 元素在 `data-[state=closed]` 上加 `[--tw-animation-fill-mode:forwards]`(必要时加 `pointer-events-none`). tw-animate-css 的 `exit` 只有 `to` 帧, fill 为 `none` 时动画结束会闪回不透明, 重子树(设置/帮助弹窗)卸载晚一帧就是"关闭闪动".
14. Radix `ScrollArea` 内部把内容包成 `display: table`: 单行截断(nowrap)的内容会把宽度撑出视口(表现为通知抽屉横向溢出), 需要 `[&_[data-slot=scroll-area-viewport]>div]:!block` 改回块布局再配合 `truncate`.
