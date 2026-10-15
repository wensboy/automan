import { staticStore } from "@/lib/static-data"
import { settingsRuntimeValues } from "@/mock/settings"
import {
  parseFieldType,
  type SettingsField,
  type SettingsGroup,
  type SettingsScalar,
  type SettingsStatic,
  type SettingsValue,
  type SettingsValues,
} from "@/static/types/settings"

/**
 * 设置的读写层 (前端唯一出口)。
 *
 * 职责边界:
 * - **描述**: 由 `static/default/settings.json` 提供 (有哪些分区/配置项、各是什么类型);
 * - **取值**: 一律由运行时下发 (后端 settings engine 叠加三视图后的生效值), **前端不参与默认值**;
 * - **提交**: 修改后交给后端落盘 (用户目录的 settings.json, schema 见 data/schema/…)。
 *
 * 当前 load/save 是 dev 实现: 值取自 `mock/settings.ts`(运行时占位) + 本地覆盖。
 * 接后端后只需替换这两个函数, 上层组件与字段形态都不用动。
 */

const STORE_KEY = "automan.settings"

/** 分区与配置项描述: 按 settings.json 的顶层键顺序组装。 */
export function settingsGroups(): SettingsGroup[] {
  const raw = (staticStore.view().Value("settings") ?? {}) as unknown as SettingsStatic
  return Object.entries(raw).flatMap(([id, fields]) => {
    const meta = settingsGroupMeta(id)
    const parsed = Object.values(fields)
      .filter((field): field is SettingsField => Boolean(field) && parseFieldType(field.type) !== undefined)
    return [{ id, label: meta.label, icon: meta.icon, fields: parsed }]
  })
}

/** 分区的展示文本与图标 (UI 文案来自 static/default/sidebar.json, 与 settings.json 的分区键对齐)。 */
function settingsGroupMeta(id: string): { label: string; icon: string } {
  const groups = staticStore.view().Value("sidebar.settings.groups") ?? []
  const hit = groups.find((group) => group.id === id)
  // 兜底: settings.json 新增分区但 sidebar 尚未补展示信息时, 用分区键本身 + 通用图标
  const fallback = id.charAt(0).toUpperCase() + id.slice(1)
  return { label: hit?.label ?? fallback, icon: hit?.icon ?? "settings" }
}

/** 按字段类型解析取值; 非法值返回 undefined (由调用方决定如何处理)。 */
export function parseFieldValue(field: SettingsField, raw: unknown): SettingsValue | undefined {
  const type = parseFieldType(field.type)
  if (!type || raw === undefined || raw === null) return undefined

  switch (type.kind) {
    case "bool":
      return typeof raw === "boolean" ? raw : undefined
    case "string":
      return typeof raw === "string" ? raw : undefined
    case "number":
      if (typeof raw !== "number" || Number.isNaN(raw)) return undefined
      // 有边界时才夹紧 (边界属描述数据, 不是默认值)
      return clamp(raw, field.type === "number" ? field.min : undefined, field.type === "number" ? field.max : undefined)
    case "enum": {
      if (!isScalarOf(type.of, raw)) return undefined
      const options = (field as { options?: unknown[] }).options ?? []
      return options.includes(raw as SettingsScalar<typeof type.of>) ? (raw as SettingsValue) : undefined
    }
    case "array": {
      if (!Array.isArray(raw)) return undefined
      return raw.every((item) => isScalarOf(type.of, item)) ? (raw as SettingsValue) : undefined
    }
  }
}

function clamp(value: number, min?: number, max?: number): number {
  let next = value
  if (typeof min === "number") next = Math.max(min, next)
  if (typeof max === "number") next = Math.min(max, next)
  return next
}

function isScalarOf(type: string, value: unknown): boolean {
  if (type === "string") return typeof value === "string"
  if (type === "number") return typeof value === "number" && !Number.isNaN(value)
  return typeof value === "boolean"
}

/**
 * 读取生效值。
 * dev: 运行时占位数据 + 本地覆盖 (逐字段校验, 非法值丢弃)。
 * 后端接入后: 直接返回 settings service 的查询结果。
 */
export async function loadSettings(): Promise<SettingsValues> {
  const values: SettingsValues = { ...settingsRuntimeValues }
  let stored: Record<string, unknown> = {}
  try {
    stored = JSON.parse(localStorage.getItem(STORE_KEY) ?? "{}") as Record<string, unknown>
  } catch {
    stored = {}
  }
  for (const group of settingsGroups()) {
    for (const field of group.fields) {
      if (!(field.key in stored)) continue
      const parsed = parseFieldValue(field, stored[field.key])
      if (parsed !== undefined) values[field.key] = parsed
    }
  }
  return values
}

/**
 * 提交修改 (dev: 写本地覆盖)。
 * 后端接入后: 提交给 settings service 并等待落盘结果; 只提交改动过的键。
 */
export async function saveSettings(values: SettingsValues): Promise<void> {
  localStorage.setItem(STORE_KEY, JSON.stringify(values))
}
