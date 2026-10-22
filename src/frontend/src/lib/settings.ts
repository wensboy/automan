import { Events } from "@wailsio/runtime"

import { SettingsService } from "../../bindings/github.com/wensboy/automan/backend/api/web"
import { staticStore } from "@/lib/static-data"
import {
  parseFieldType,
  type SettingsField,
  type SettingsFieldEntry,
  type SettingsGroup,
  type SettingsStatic,
  type SettingsValue,
  type SettingsValues,
} from "@/static/types/settings"

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
  return Object.entries(raw).flatMap(([id, node]) => {
    const meta = settingsGroupMeta(id)
    return [{ id, label: meta.label, icon: meta.icon, fields: collectFields(node) }]
  })
}

/** 判断一个节点是否是配置项(有可解析的 type), 用于与"子分组"区分。 */
function isFieldNode(node: unknown): boolean {
  if (!node || typeof node !== "object" || Array.isArray(node)) return false
  const candidate = node as { key?: unknown; label?: unknown; type?: unknown }
  // 必须是**完整字段**: 缺 key/label 的节点(例如旧格式里被包在 value 下的 {type,options})
  // 不能当字段渲染, 否则 label 为空会在渲染期直接崩(field.label.split)。
  if (typeof candidate.key !== "string" || typeof candidate.label !== "string") return false
  return typeof candidate.type === "string" && parseFieldType(candidate.type) !== undefined
}

/**
 * 递归收集分区下的配置项 —— 允许分组嵌套(例 `general.session.expirationS`),
 * 层级只影响组织方式(配置项 key 始终是完整点号键); 子分组键记进 `trail`, 供 UI 用 `>` 展示。
 */
function collectFields(node: unknown, trail: string[] = []): SettingsFieldEntry[] {
  if (!node || typeof node !== "object" || Array.isArray(node)) return []
  const fields: SettingsFieldEntry[] = []
  for (const [name, value] of Object.entries(node as Record<string, unknown>)) {
    if (isFieldNode(value)) {
      fields.push({ ...(value as SettingsField), trail })
      continue
    }
    fields.push(...collectFields(value, [...trail, name]))
  }
  return fields
}

/** 取出 number 字段的 [min, max]: 仅当 `range` 恰为两个数字时有效, 否则视为无边界。 */
export function rangeBounds(field: SettingsField): { min?: number; max?: number } {
  if (field.type !== "number") return {}
  const range = Array.isArray(field.range) ? field.range : []
  if (range.length !== 2) return {}
  const [min, max] = range
  if (typeof min !== "number" || typeof max !== "number") return {}
  return { min, max }
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
    case "number": {
      if (typeof raw !== "number" || Number.isNaN(raw)) return undefined
      const { min, max } = rangeBounds(field)
      return clamp(raw, min, max)
    }
    case "enum": {
      if (!isScalarOf(type.of, raw)) return undefined
      const options = (field as { options?: unknown }).options
      // options 为字符串 = **动态来源键**(候选由后端 Options(key) 下发):
      // 前端不做白名单校验(拿不到列表, 用字符串 includes 会误判), 交给后端 Set 校验。
      if (typeof options === "string") return raw as SettingsValue
      const list = Array.isArray(options) ? options : []
      return list.includes(raw as never) ? (raw as SettingsValue) : undefined
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
