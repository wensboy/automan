---
name: "backend workflow"
description: "backend 必要的规范"
---

## 规范

1. service 总是编排 engine, 通过 domain 接口对engine实例做不同能力调用.
2. service 多 package 操作冗余的情况下, 应该将逻辑抽离到 internal 下作为独立的 toolkit 包, 通常具备单一职责, 命名尽可能语义化.
3. 不要过渡抽象, 不要不抽象.
4. service 之间调用通过方法/函数闭包, 避免服务之间过渡调用和注入. 
5. 独立领域新开语义 domain, 避免 domian 职责边界混乱.
6. 针对后端而言, 所有程序配置在 `src/data/conf/*.json`; 默认数据在 `src/data/default`; 文档数据在 `src/data/docs/`; 数据库迁移在 `src/data/migrate/*.sql`; 所有规范结构在 `src/data/schema`.

## 细节

1. 在能够用原生库和engine的情况下, 优先engine, 例如: 文件系统操作应该统一走 store engine, 除非 engine 做不到. 
2. internal 下的 package name 不允许以common, util, kit 等命名, 需要实际语义化, 不允许模糊语义.