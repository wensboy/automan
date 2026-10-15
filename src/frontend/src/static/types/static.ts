import authDefault from "@/static/default/auth.json"
import contentAreaDefault from "@/static/default/contentArea.json"
import settingsDefault from "@/static/default/settings.json"
import sidebarDefault from "@/static/default/sidebar.json"

/**
 * 静态数据 (layer view) 的类型定义。
 *
 * 数据分层: `static/default/*.json` 是 base 层, `static/cover/*.json` 是 cover 层 ——
 * 读取时按 key 逐层合并 (path 相同即覆盖, 例 `a.b.c`), base 只读、cover 用于部署期覆盖。
 * 访问统一走 json key (点号路径), 见 `StaticView.Value`。
 */

export type JsonPrimitive = string | number | boolean | null
export type JsonValue = JsonPrimitive | JsonValue[] | { [key: string]: JsonValue }
export type JsonObject = { [key: string]: JsonValue }

/**
 * 组装后的原始数据形态: **文件名即 root 的一层字段**, 目录层级用点号连接
 * (例 `static/default/orders/detail.json` -> `orders.detail`)。
 * 新增文件时在这里补一条字段 —— 结构从 json 自身推导, 不手写。
 */
export type StaticShape = {
  sidebar: typeof sidebarDefault
  contentArea: typeof contentAreaDefault
  auth: typeof authDefault
  settings: typeof settingsDefault
}

/**
 * 可访问的 json key (点号路径)。
 * - 对象继续展开, 因此**非叶子节点也可访问** (`Value("sidebar.user")` 拿到对象);
 * - 数组与原始值视为叶子, 整块取用 (与合并策略一致: 数组整体替换, 不按下标合并) ——
 *   批量渲染的列表 (`[]`) 与整体传递的对象 (`{}`) 都放在 end 节点上。
 */
export type DotPaths<T> = T extends JsonPrimitive | readonly unknown[]
  ? never
  : {
      [K in keyof T & string]: T[K] extends JsonPrimitive | readonly unknown[]
        ? K
        : K | `${K}.${DotPaths<T[K]>}`
    }[keyof T & string]

/**
 * 由 json key 推导出的值类型。
 * key 可写任意字符串 (运行期缺失返回 undefined); 已声明的 path 会得到精确类型,
 * 未声明/写错的 path 退回 JsonValue。
 */
export type ValueAt<T, P extends string> = P extends `${infer Head}.${infer Rest}`
  ? Head extends keyof T
    ? ValueAt<T[Head], Rest>
    : JsonValue
  : P extends keyof T
    ? T[P]
    : JsonValue

/** 已声明的 key (点号路径), 便于取值时补全。 */
export type StaticPath = DotPaths<StaticShape>
export type StaticValue<P extends string> = ValueAt<StaticShape, P>

/** 变更片段: 结构按 key 可选; 数组与原始值整块给出 (替换语义)。 */
export type DeepPartial<T> = T extends JsonPrimitive | readonly unknown[]
  ? T
  : { [K in keyof T]?: DeepPartial<T[K]> }

export type StaticPatch = DeepPartial<StaticShape>

/**
 * 层视图 (layer view): 统一通过 json key 访问值, 并提供两种变更方式。
 * - `shadow(patch)`: **临时**变更, 返回新视图; 只影响该视图及其派生视图的读取结果, 不修改任何数据;
 *   可叠加且**内层掩盖外层** (与语言的局部作用域同构)。
 * - `cover(patch)`: **永久**变更, 叠加到 cover 层; 所有视图都可见, 并触发订阅者重新渲染。
 */
export type StaticView = {
  /** 按 json key 取值; key 不存在时返回 undefined (不抛异常, 由调用方决定如何处理)。 */
  Value<P extends string>(path: P): StaticValue<P> | undefined
  shadow(patch: StaticPatch): StaticView
  cover(patch: StaticPatch): void
}
