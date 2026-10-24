import * as React from "react"

import LogFilters from "@/components/custom/logs/filters"
import { logRangeMs, type LogLevel, type LogTimeRange } from "@/components/custom/logs/model"
import LogTerminal from "@/components/custom/logs/terminal"
import { clearLogs, ensureLogSubscription, snapshotLogs, subscribeLogs } from "@/lib/logs"

/**
 * 日志 (nav-Main: Logs): 头部筛选 + 终端显示 + 命令输入。
 * 页面持有数据与业务状态 (日志缓冲 / 搜索 / 级别 / 时间范围), 组件只负责渲染。
 * 数据来自后端日志引擎的二级总线事件 "log:stream" (只含程序运行启动后的日志)。
 */
export default function Logs() {
  // 行缓冲在模块层(lib/logs): 路由切换不清空, 只有显式 clear 才丢弃
  const lines = React.useSyncExternalStore(subscribeLogs, snapshotLogs)
  const [query, setQuery] = React.useState("")
  const [level, setLevel] = React.useState<LogLevel | "all">("all")
  const [range, setRange] = React.useState<LogTimeRange>("all")

  React.useEffect(() => {
    ensureLogSubscription()
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
          onClear={clearLogs}
        />
        <LogTerminal lines={visible} className="min-h-0 flex-1" />
      </div>
    </div>
  )
}
