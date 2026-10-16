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
  level: string
  msg: string
  fields?: { key: string; value: string }[]
}

const LEVELS: LogLevel[] = ["debug", "info", "warn", "error"]

function toLevel(raw: string): LogLevel {
  const level = raw?.toLowerCase() as LogLevel
  return LEVELS.includes(level) ? level : "info"
}

/** 附加字段拼进消息 (与落盘格式一致: ` key=value`)。 */
function formatMessage(record: StreamRecord): string {
  let message = record.msg ?? ""
  for (const field of record.fields ?? []) {
    message += ` ${field.key}=${field.value}`
  }
  return message
}

/**
 * 终端行首时间戳: `YYYY-MM-DD HH:mm:ss` (本地时区)。
 * 日志常跨天查看, 只给时分秒无法区分日期, 所以年月日一并显示。
 */
export function formatLogTime(ms: number): string {
  const date = new Date(ms)
  const pad = (value: number) => String(value).padStart(2, "0")
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    ` ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  )
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
