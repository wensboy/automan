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
7. 所有 `data/` 下的 json 结构可以直接使用原生的 text/template 解析 `{{.xxx}}` 结构, 通常满足: 该字段无法编译时确定, 运行时操作替换, 替换后通常固定; `*.tpl.*` 模板文件(显式, 通常存在 `{{}}` 结构). 结构内字段通常采用大驼峰格式命名, template使用一般结构体映射, 对于复杂场景下考虑用map避免过度定义结构. 当前不考虑 function, condition 等高级处理形式.

## 细节

1. 在能够用原生库和engine的情况下, 优先engine, 例如: 文件系统操作应该统一走 store engine, 除非 engine 做不到. 
2. internal 下的 package name 不允许以common, util, kit 等命名, 需要实际语义化, 不允许模糊语义.
3. 对于同类型包下的不同领域模块, 在 import 导入时总是以: `<module><domain>` 命名导入, 不依赖默认导入. 例如: service/ 下的所有服务导入时都别名为 `sercicexxx`, 比如: `serviceproxy, servicesettings, servicehelp` 等, engine, domain 同理.
4. service 构造函数统一 `New()` 无参, 依赖经 `WithX()` 链式注入并返回自身; bootstrap 只做 `New()` + `With` 串写.
5. 所有 sqlx 扫描的结构体必须写 `db` tag(与查询列名/别名一致); 缺 tag 会报 `missing destination name`.
6. 文件读写必须走 store engine(`CmdRead/Write/Mkdir/Remove/Stat/List`), 路径为 store 根下相对路径, 拒绝绝对路径与 `..`; 原子写由引擎保证.
7. 服务文件按职责拆分(如 CRUD / store 适配 / 树操作 / 文档解析 / 路径校验), 单文件单一职责并在文件首行注明职责.
8. 内嵌数据根包含 `data/` 前缀(`//go:embed data`), 读取模板/默认文件时必须写成 `data/default/...`.
9. 运行时配置优先, 构建注入兜底; 注入键统一 `-X .../domain/config.buildVersion|buildTime|buildHash`.
10. 读多写少服务使用 RWMutex + 按路径/键缓存失效; 写操作持锁完成并精确失效相关前缀.
11. 破坏性操作默认安全: 删目录仅允许空目录, 递归需显式参数且受配置开关(如 `allowRecursiveDelete`)控制; `RowsAffected == 0` 必须视为失败, 不得继续发成功提示.
12. 作用域 SQL 的过滤条件统一**追加**在 where 末尾, 保证参数顺序与占位符一致(避免 user_id/id 错位).
13. 设计文档(vxx.md)只记录需求与决策历史; 实现中沉淀的通用规范写入本文件 `## 细节`.
14. 模板文件(`*.tpl.*`)里的 `{{.Xxx}}` 一律用标准库 `text/template` 渲染, 禁止手写正则/字符串替换; 上下文用小写私有 struct(字段名即模板变量名), 解析失败归 `ErrInvalidFile`、未知字段归 `ErrUnknownPlace`; 类似 `${X}` 的**字符串占位符**(如启动参数)仍走 `internal/flat`, 不套模板.
