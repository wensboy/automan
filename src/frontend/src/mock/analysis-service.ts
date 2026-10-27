/**
 * Service 视图的展示辅助(纯类型与格式化, 不含数据)。
 *
 * 数据来源: 后端 ServiceSnapshot(见 lib/analysis.ts);
 * 这里只保留系列定义类型、采样间隔与单位格式化, 避免组件里散落格式逻辑。
 */

export type ServiceResource = "cpu" | "ram" | "io" | "gc" | "heap" | "goroutine" | "bridge"

export type SeriesDef = {
  key: string
  label: string
  unit: "%" | "MB" | "count" | "ms"
  digits?: number
}

export type Sample = {
  ts: number
  values: Record<string, number>
}

/** 采样间隔(与后端 conf/analysis.sampleIntervalMs 对齐)。 */
export const serviceIntervalMs = 10_000

/** 进程号占位: 由后端快照提供, 这里仅作为 footer 缺省显示。 */
export const servicePid = 0

/** 格式化一个指标值(带单位)。 */
export function formatMetric(value: number, series: SeriesDef): string {
  const digits = series.digits ?? 0
  const number = value.toFixed(digits)
  if (series.unit === "%") return `${number}%`
  if (series.unit === "count") return number
  return `${number} ${series.unit}`
}
