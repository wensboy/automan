import { createStaticStore } from "@/lib/static-store"
import type { JsonObject } from "@/static/types/static"

/**
 * 静态数据的组装入口: 构建期汇总 `static/default/**` 与 `static/cover/**` 下的所有 json。
 * 文件路径即 key (目录层级用点号连接), 因此**新增/拆分文件无需改代码**, 只在本文件声明类型
 * (见 `static/types/static.ts` 的 StaticShape)。
 */

const isPlainObject = (value: unknown): value is JsonObject =>
  typeof value === "object" && value !== null && !Array.isArray(value)

/** 把 `../static/<layer>/a/b.json` 汇总成 `{ a: { b: <json> } }`。 */
function collectJson(modules: Record<string, { default: JsonObject }>, marker: string): JsonObject {
  const root: JsonObject = {}
  for (const [file, mod] of Object.entries(modules)) {
    const segments = file
      .slice(file.indexOf(marker) + marker.length)
      .replace(/\.json$/, "")
      .split("/")

    let cursor = root
    for (const segment of segments.slice(0, -1)) {
      if (!isPlainObject(cursor[segment])) {
        cursor[segment] = {}
      }
      cursor = cursor[segment] as JsonObject
    }
    cursor[segments[segments.length - 1]] = mod.default
  }
  return root
}

export const staticStore = createStaticStore({
  base: collectJson(import.meta.glob("../static/default/**/*.json", { eager: true }), "/default/"),
  cover: collectJson(import.meta.glob("../static/cover/**/*.json", { eager: true }), "/cover/"),
})
