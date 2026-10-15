/**
 * 日志页的开发期数据: 级别/时间范围选项 + 日志行生成。
 * 接入真实日志源后, 页面改为订阅该源即可 (组件不动)。
 */

export type LogLevel = "debug" | "info" | "warn" | "error"

export type LogLine = {
  id: number
  time: number
  level: LogLevel
  message: string
}

export const logLevels: { value: LogLevel; label: string }[] = [
  { value: "debug", label: "Debug" },
  { value: "info", label: "Info" },
  { value: "warn", label: "Warn" },
  { value: "error", label: "Error" },
]

export const logTimeRanges = [
  { value: "all", label: "All time" },
  { value: "5m", label: "Last 5 min" },
  { value: "15m", label: "Last 15 min" },
  { value: "1h", label: "Last hour" },
] as const

export type LogTimeRange = (typeof logTimeRanges)[number]["value"]

/** 时间范围 -> 毫秒 (all 之外的部分)。 */
export const logRangeMs: Record<Exclude<LogTimeRange, "all">, number> = {
  "5m": 5 * 60_000,
  "15m": 15 * 60_000,
  "1h": 60 * 60_000,
}

const samples: { level: LogLevel; message: string }[] = [
  { level: "info", message: "bridge: listening sock=bridge/node.sock codec=json" },
  { level: "info", message: "bridge: child started command=node pid=24117" },
  { level: "debug", message: "ipc: frame received id=7 bytes=128" },
  { level: "info", message: "settings: configuration written to store" },
  { level: "warn", message: "ipc: write queue is filling up (48/64)" },
  { level: "error", message: "bridge: handshake rejected: version mismatch" },
  { level: "debug", message: "store: cache hit key=settings.json" },
  { level: "info", message: "proxy: profile \"local-dev\" applied (4 rules)" },
  { level: "warn", message: "capture: buffer reached 92% of limit" },
  { level: "info", message: "automation: run #128 finished (11 ok, 1 skipped)" },
  { level: "debug", message: "database: vacuum scheduled in 30m" },
  { level: "error", message: "bridge: child exited with code 1, restarting" },
]

let cursor = 0

/** 生成一条日志 (级别/内容按顺序循环, 保证输出可预期)。 */
export function createLogLine(id: number): LogLine {
  const sample = samples[cursor % samples.length]
  cursor += 1
  return { id, time: Date.now(), level: sample.level, message: sample.message }
}
