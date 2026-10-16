import * as React from "react"
import { Events } from "@wailsio/runtime"

import LogFilters from "@/components/custom/logs/filters"
import { logRangeMs, toLogLines, type LogLevel, type LogLine, type LogTimeRange, type StreamRecord } from "@/components/custom/logs/model"
import LogTerminal from "@/components/custom/logs/terminal"

/** 本地行缓冲上限 (超出丢弃最旧)。 */
const MAX_LINES = 500

/**
 * 日志 (nav-Main: Logs): 头部筛选 + 终端显示 + 命令输入。
 * 页面持有数据与业务状态 (日志缓冲 / 搜索 / 级别 / 时间范围), 组件只负责渲染。
 * 数据来自后端日志引擎的二级总线事件 "log:stream" (只含程序运行启动后的日志)。
 */
export default function Logs() {
  const [lines, setLines] = React.useState<LogLine[]>([])
  const [query, setQuery] = React.useState("")
  const [level, setLevel] = React.useState<LogLevel | "all">("all")
  const [range, setRange] = React.useState<LogTimeRange>("all")

  React.useEffect(() => {
    let nextId = 0
    // 后端已按批量窗口下发(默认 120ms), 这里只做追加与上限裁剪, 不再逐条渲染
    return Events.On("log:stream", (event) => {
      const records = event.data as StreamRecord[] | undefined
      if (!Array.isArray(records) || records.length === 0) return
      setLines((prev) => {
        const appended = toLogLines(records, nextId)
        nextId += appended.length
        const next = [...prev, ...appended]
        return next.length > MAX_LINES ? next.slice(next.length - MAX_LINES) : next
      })
    })
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
