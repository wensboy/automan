import { Events } from "@wailsio/runtime"

import { SettingsService } from "../../bindings/github.com/wensboy/automan/backend/api/web"
import { staticStore } from "@/lib/static-data"
import { parseFieldType, type SettingsField, type SettingsGroup, type SettingsStatic, type SettingsValue, type SettingsValues } from "@/static/types/settings"

/**
 * 设置的读写层 (前端唯一出口)。
 *
 * 职责边界:
 * - **描述**: `static/default/settings.json` 决定有哪些分区/配置项、各是什么类型(控件映射见 settings-dialog);
 * - **取值**: 一律来自后端 settings 服务(引擎三视图 runtime > user > bottom 叠加后的生效值),
 *   前端**不参与默认值**; 值缺失时展示占位符, 不臆造;
 * - **提交**: `Apply` 写运行态 + `Save` 落盘到用户目录的 settings.json;
 * - **回流**: 订阅后端事件(落盘完成 / 外部改动重载)后重新拉取。
 */
export function settingsGroups(): SettingsGroup[] {
  const raw = (staticStore.view().Value("settings") ?? {}) as unknown as SettingsStatic
  return Object.entries(raw).flatMap(([id, fields]) => {
    const meta = settingsGroupMeta(id)
    const parsed = Object.values(fields).filter(
      (field): field is SettingsField => Boolean(field) && parseFieldType(field.type) !== undefined,
    )
    return [{ id, label: meta.label, icon: meta.icon, fields: parsed }]
  })
}

/** 分区 -> 配置项 的查找表 (按 key 取字段描述)。 */
function fieldByKey(): Map<string, SettingsField> {
  const index = new Map<string, SettingsField>()
  for (const group of settingsGroups()) {
    for (const field of group.fields) index.set(field.key, field)
  }
  return index
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
      return clamp(raw, field.type === "number" ? field.min : undefined, field.type === "number" ? field.max : undefined)
    case "enum": {
      if (!isScalarOf(type.of, raw)) return undefined
      const options = (field as { options?: unknown[] }).options ?? []
      return options.includes(raw as never) ? (raw as SettingsValue) : undefined
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
 * 读取生效值: 一次 List 拿全量, 只保留后端下发的值(前端不补默认值)。
 * 后端若给出与描述不符的类型, 这里退回原值, 由控件层展示占位/降级。
 */
export async function loadSettings(): Promise<SettingsValues> {
  const entries = (await SettingsService.List()) ?? {}
  const fields = fieldByKey()
  const values: SettingsValues = {}
  for (const [key, entry] of Object.entries(entries)) {
    if (!entry) continue
    const field = fields.get(key)
    values[key] = (field ? parseFieldValue(field, entry.value) : undefined) ?? (entry.value as SettingsValue)
  }
  return values
}

/** 提交修改: Apply(运行态) + Save(落盘)。只提交改动过的键。 */
export async function saveSettings(patch: SettingsValues): Promise<void> {
  if (Object.keys(patch).length === 0) return
  await SettingsService.Apply(patch)
  await SettingsService.Save()
}

/** 抹掉某键的覆盖并落盘(回落默认层)。 */
export async function resetSetting(key: string): Promise<void> {
  await SettingsService.Reset(key)
  await SettingsService.Save()
}

export type SettingsEventKind = "changed" | "saved" | "reloaded"

/**
 * 订阅后端设置事件, 返回取消函数。
 * - saved   : 本次落盘完成
 * - reloaded: 用户文件被外部修改后重载
 * - changed : 运行态单键变更(我们的批量提交也会触发, 界面可自行取舍)
 */
export function subscribeSettings(handler: (kind: SettingsEventKind) => void): () => void {
  const offs = [
    Events.On("settings:changed", () => handler("changed")),
    Events.On("settings:saved", () => handler("saved")),
    Events.On("settings:reloaded", () => handler("reloaded")),
  ]
  return () => {
    for (const off of offs) off()
  }
}
