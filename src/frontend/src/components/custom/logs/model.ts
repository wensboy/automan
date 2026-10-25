/**
 * 日志页的领域模型: 终端行形态 + 时间范围换算 + 后端记录到行的映射。
 * 后端事件 "log:stream" 的载荷即 Record 的 JSON 形态(见 docs/design/v16.md 第 6 节)。
 */

export type LogLevel = "debug" | "info" | "warn" | "error"

export type LogLine = {
  id: number
  /** 毫秒时间戳 (终端按本地时间显示)。 */
  time: number
  level: LogLevel
  message: string
}

export type LogTimeRange = "all" | "5m" | "15m" | "1h"

/** 时间范围 -> 毫秒 (all 之外的部分)。 */
export const logRangeMs: Record<Exclude<LogTimeRange, "all">, number> = {
  "5m": 5 * 60_000,
  "15m": 15 * 60_000,
  "1h": 60 * 60_000,
}

/** 后端下发的单条记录 (Record 的 JSON 形态)。 */
export type StreamRecord = {
  time: string
  /** 后端 Level 是 Go 枚举: 绑定层生成的是数字(0..3), 也兼容字符串形态。 */
  level: string | number
  msg: string
  fields?: { key: string; value: any }[] | null
}

const LEVELS: LogLevel[] = ["debug", "info", "warn", "error"]

function toLevel(raw: unknown): LogLevel {
  // 数字枚举: 0=debug 1=info 2=warn 3=error(见 domain/log.Level)
  if (typeof raw === "number") {
    return LEVELS[raw] ?? "info"
  }
  const level = String(raw ?? "").toLowerCase() as LogLevel
  return LEVELS.includes(level) ? level : "info"
}

/** 附加字段拼进消息 (与落盘格式一致: ` key=value`)。 */
function formatMessage(record: StreamRecord): string {
  let message = record.msg ?? ""
  for (const field of record.fields ?? []) {
    message += ` ${field.key}=${String(field.value)}`
  }
  return message
}

/**
 * 把一批后端记录映射为终端行; startId 为自增起点(行 id 由页面维护)。
 * 时间无法解析时退回当前时间, 保证行仍可见。
 */
export function toLogLines(records: StreamRecord[], startId: number): LogLine[] {
  return records.map((record, index) => {
    const parsed = Date.parse(record.time)
    return {
      id: startId + index + 1,
      time: Number.isNaN(parsed) ? Date.now() : parsed,
      level: toLevel(record.level),
      message: formatMessage(record),
    }
  })
}
