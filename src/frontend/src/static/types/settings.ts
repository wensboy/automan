/**
 * 设置项的类型定义 (前端侧契约)。
 *
 * `static/default/settings.json` 只描述**有哪些配置项、各是什么形态** ——
 * 顶层键即分区, 二级键即配置项, 每项给 `key/label/description/type` (+ 类型专属键)。
 *
 * 类型规则(严格): 标量直接写 `bool` / `string` / `number`; 复合类型**必须带 `<T>`**,
 * 便于后续按 T 解析、校验与调用接口:
 *   `enum<string>`  -> 取值必须落在 options 内        (控件: select, 非对齐)
 *   `array<string>` -> 元素必须符合 T 的数组           (控件: list-input)
 *
 * **前端不关心默认值**: 生效值一律由运行时下发(后端 settings engine 叠加后返回),
 * 前端只负责展示与提交修改。用户文件的 schema 见 `data/schema/settings.user.schema.json`。
 */

/** `<T>` 允许的标量类型。 */
export type SettingsScalarType = "string" | "number" | "boolean"

/** `<T>` 对应的 TS 类型。 */
export type SettingsScalar<T extends SettingsScalarType> =
  T extends "string" ? string : T extends "number" ? number : boolean

/** 字段类型字面量: 标量原样, enum/array 必须带 `<T>`。 */
export type SettingsFieldType =
  | "bool"
  | "string"
  | "number"
  | `enum<${SettingsScalarType}>`
  | `array<${SettingsScalarType}>`

export type SettingsFieldBase = {
  /** 点号路径, 与 settings engine 的键一致 (例 appearence.mode)。 */
  key: string
  label: string
  description?: string
}

export type SettingsBoolField = SettingsFieldBase & { type: "bool" }

export type SettingsStringField = SettingsFieldBase & { type: "string" }

export type SettingsNumberField = SettingsFieldBase & {
  type: "number"
  /** 给定时用滑杆并夹紧取值范围; 缺省时退化为数字输入。 */
  min?: number
  max?: number
  step?: number
}

export type SettingsEnumField<T extends SettingsScalarType = SettingsScalarType> =
  SettingsFieldBase & {
    type: `enum<${T}>`
    options: SettingsScalar<T>[]
  }

export type SettingsArrayField<T extends SettingsScalarType = SettingsScalarType> =
  SettingsFieldBase & {
    type: `array<${T}>`
  }

/** 判别联合: 按 field.type 收窄。 */
export type SettingsField =
  | SettingsBoolField
  | SettingsStringField
  | SettingsNumberField
  | SettingsEnumField
  | SettingsArrayField

/** 解析后的类型信息 (把 `enum<string>` / `array<number>` 拆成 kind + 元素类型)。 */
export type ParsedFieldType =
  | { kind: "bool" }
  | { kind: "string" }
  | { kind: "number" }
  | { kind: "enum"; of: SettingsScalarType }
  | { kind: "array"; of: SettingsScalarType }

/** 解析字段类型; 无法识别时返回 undefined (视为数据配置错误)。 */
export function parseFieldType(type: string): ParsedFieldType | undefined {
  if (type === "bool" || type === "string" || type === "number") {
    return { kind: type }
  }
  const match = /^(enum|array)<(string|number|boolean)>$/.exec(type)
  if (!match) return undefined
  return { kind: match[1] as "enum" | "array", of: match[2] as SettingsScalarType }
}

/** 单个配置项的取值。 */
export type SettingsValue = string | number | boolean | string[] | number[] | boolean[]

/** 运行期取值集合: key -> 值 (由运行时下发)。 */
export type SettingsValues = Record<string, SettingsValue>

/** 一个分区 (顶层键) 及其配置项 —— 由 settings.json 组装而成。 */
export type SettingsGroup = {
  id: string
  label: string
  icon: string
  fields: SettingsField[]
}

/** settings.json 的原始形态: 分区 -> 配置项名 -> 字段描述。 */
export type SettingsStatic = Record<string, Record<string, SettingsField>>
