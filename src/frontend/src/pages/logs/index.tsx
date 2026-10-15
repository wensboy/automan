import * as React from "react"

import LogFilters from "@/components/custom/logs/filters"
import LogTerminal from "@/components/custom/logs/terminal"
import {
  createLogLine,
  logRangeMs,
  type LogLevel,
  type LogLine,
  type LogTimeRange,
} from "@/mock/logs"

const WRITE_INTERVAL_MS = 700
const MAX_LINES = 500

/**
 * 日志 (nav-Main: Logs): 头部筛选 + 终端显示 + 命令输入。
 * 页面持有数据与业务状态 (日志缓冲 / 搜索 / 级别 / 时间范围), 组件只负责渲染。
 * dev 阶段日志由定时器持续写入, 接真实日志源后替换这一处即可。
 */
export default function Logs() {
  const [lines, setLines] = React.useState<LogLine[]>([])
  const [query, setQuery] = React.useState("")
  const [level, setLevel] = React.useState<LogLevel | "all">("all")
  const [range, setRange] = React.useState<LogTimeRange>("all")

  React.useEffect(() => {
    let nextId = 0
    const timer = window.setInterval(() => {
      nextId += 1
      setLines((prev) => {
        const next = [...prev, createLogLine(nextId)]
        return next.length > MAX_LINES ? next.slice(next.length - MAX_LINES) : next
      })
    }, WRITE_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [])

  const visible = React.useMemo(() => {
    const keyword = query.trim().toLowerCase()
    const since = range === "all" ? 0 : Date.now() - logRangeMs[range]
    return lines.filter((line) => {
      if (level !== "all" && line.level !== level) return false
      if (since !== 0 && line.time < since) return false
      if (keyword !== "" && !line.message.toLowerCase().includes(keyword)) return false
      return true
    })
  }, [lines, query, level, range])

  return (
    <div className="flex min-h-0 flex-1 flex-col p-4 pt-0">
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border">
        <LogFilters
          className="border-b"
          query={query}
          onQueryChange={setQuery}
          level={level}
          onLevelChange={setLevel}
          range={range}
          onRangeChange={setRange}
        />
        <LogTerminal lines={visible} className="min-h-0 flex-1" />
      </div>
    </div>
  )
}
