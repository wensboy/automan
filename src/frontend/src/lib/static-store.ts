import type { JsonObject, JsonValue, StaticPatch, StaticValue, StaticView } from "@/static/types/static"

/**
 * 静态数据 layer view 的核心实现 (纯函数, 不依赖构建器/React, 便于单独验证)。
 * 组装数据源见 `lib/static-data.ts`; React 接线见 `share/provider/static-provider.tsx`。
 */

const isPlainObject = (value: unknown): value is JsonObject =>
  typeof value === "object" && value !== null && !Array.isArray(value)

/**
 * 逐层叠加规则: 对象按 key 递归合并 (path 相同即覆盖), 数组与原始值**整体替换**。
 * 只读实现 —— 始终返回新对象, 因此 shadow/cover 都不会改动已有层。
 */
export function mergeDeep(base: JsonValue | undefined, patch: unknown): JsonValue {
  // patch 允许传入带可选字段的"变更片段"(如 DeepPartial<StaticShape>), 因此形参放宽为 unknown;
  // 非对象(或数组)一律整体替换, 对象则逐键递归。
  if (!isPlainObject(base) || !isPlainObject(patch)) return patch as JsonValue
  const next: JsonObject = { ...base }
  for (const [key, value] of Object.entries(patch)) {
    next[key] = key in base ? mergeDeep(base[key], value) : value
  }
  return next
}

/** 按 json key (点号路径) 取值; 缺失时返回 undefined (类型保证下不应发生)。 */
export function readPath(root: JsonValue, path: string): JsonValue | undefined {
  let cursor: JsonValue | undefined = root
  for (const key of path.split(".")) {
    if (!isPlainObject(cursor)) return undefined
    cursor = cursor[key]
  }
  return cursor
}

export type StaticStore = {
  /** 根视图 (无 shadow)。 */
  view: () => StaticView
  subscribe: (listener: () => void) => () => void
  /** 供 useSyncExternalStore 使用: cover 后版本号变化。 */
  snapshot: () => number
}

/**
 * 创建静态数据容器: base 为底, cover 层可被覆盖, shadow 只在各自视图内生效。
 */
export function createStaticStore({ base, cover = {} }: { base: JsonObject; cover?: JsonObject }): StaticStore {
  let coverLayer: JsonObject = cover
  let version = 0
  const listeners = new Set<() => void>()

  // 读数 = base + cover + 该视图的 shadow 链 (内层在后, 因此掩盖外层)
  const layered = (shadows: JsonObject[]): JsonObject =>
    shadows.reduce<JsonObject>(
      (acc, patch) => mergeDeep(acc, patch) as JsonObject,
      mergeDeep(base, coverLayer) as JsonObject,
    )

  const createView = (shadows: JsonObject[]): StaticView => ({
    Value: <P extends string>(path: P): StaticValue<P> | undefined =>
      readPath(layered(shadows), path) as StaticValue<P> | undefined,
    shadow: (patch: StaticPatch) => createView([...shadows, patch as JsonObject]),
    cover: (patch: StaticPatch) => {
      coverLayer = mergeDeep(coverLayer, patch) as JsonObject
      version += 1
      for (const listener of listeners) listener()
    },
  })

  const rootView = createView([])

  return {
    view: () => rootView,
    subscribe: (listener) => {
      listeners.add(listener)
      return () => {
        listeners.delete(listener)
      }
    },
    snapshot: () => version,
  }
}
