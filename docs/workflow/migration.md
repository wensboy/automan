---
name: "migration workflow"
description: "migrate 链路统一处理思路和细节"
---

## 流程

1. data/migrate/ 下按照序号和schema规范(vxx.md通常为唯一真实来源)定义 migrate sql, 包含 up, down.
2. 将 up, down 通过 engine/database/migrate.go 注册.
3. 按照实际校验和测试 migrate.

## 细节

1. 总是需要考虑 migrate 链路是否符合预期, 避免不必要的问题出现.