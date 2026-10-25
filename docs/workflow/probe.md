---
name: "probe workflow"
description: "probe 探测查 bug 问题. 主要针对前端."
---

## 流程

1. 利用探针([probe]标识的关键上下文信息)在验证链路源代码关键位置插入.
2. 要求用户按照指定操作走链路, 并关注probe相关信息.
3. 用户将 probe 信息传递后根据 probe 信息调整下一步是否继续 probe.
4. probe 为有限步操作, 最终通过决定性数据定位问题并处理.

## 细节

1. probe 定义时就应该考虑到删除, 避免探针遗漏.
2. probe 只取关键链路的关键信息.
3. 禁止无限 probe.
