---
name: "frontend workflow"
description: "frontend 必要的规范"
---

## 规范

1. 所有静态数据在 `frontend/src/static/` 下, 以高度语义化的形式存储为json对象.
2. 所有组件定义到 `frontend/src/components/custom/` 下; 共享组件定义到 `frontend/src/components/custom/share/` 下.
3. 页面定义到 `frontend/src/pages/` 下; 布局定义到 `frontend/src/layouts/` 下.
4. 在完善 ui 时总是需要考虑将 mock 数据定义到 `frontend/src/mock/` 下, 对接完成后测试并移除 mock.
5. 总是优先使用 shadcn/ui 原生组件. 考虑最大 3 轮无法达到效果使用自定义组件处理.
6. 保持界面风格和类似组件风格显示始终一致.

## 细节

1. 静态数据通常包含: ui组件静态文本, ui组件动态文本映射, 组件显示定义, 组件静态类型定义, 灵活拓展覆盖.