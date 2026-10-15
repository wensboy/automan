import * as React from "react"
import { FitAddon } from "@xterm/addon-fit"
import { Terminal } from "@xterm/xterm"
import { cn } from "cn"

import RichInput from "@/components/custom/share/rich-input"

import type { LogLine } from "@/mock/logs"

import "@xterm/xterm/css/xterm.css"

import "./terminal.css"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/** 级别 -> ANSI 颜色 (用终端自带的调色板, 不写死 hex)。 */
const levelColor: Record<LogLine["level"], string> = {
  debug: "\x1b[90m",
  info: "\x1b[0m",
  warn: "\x1b[33m",
  error: "\x1b[31m",
}

const RESET = "\x1b[0m"

function formatLine(line: LogLine): string {
  const time = new Date(line.time).toTimeString().slice(0, 8)
  const level = line.level.toUpperCase().padEnd(5)
  const prefix = levelColor[line.level]
  return `${time} ${prefix}${level}${RESET} ${line.message}`
}

/**
 * 日志终端: xterm 显示区 + 底部命令输入。
 * lines 为页面筛选后的结果 —— 追加时只写增量, 筛选变化时整体重写 (保留历史滚动)。
 */
export default function LogTerminal({
  lines,
  className,
}: {
  lines: LogLine[]
  className?: string
}) {
  const hostRef = React.useRef<HTMLDivElement | null>(null)
  const termRef = React.useRef<Terminal | null>(null)
  const renderedRef = React.useRef<LogLine[]>([])
  const { Value } = useStatic()
  const [command, setCommand] = React.useState("")

  // 初始化终端 (挂载一次); 主题色取 token —— xterm 不解析 oklch, 故 token 用可解析格式。
  React.useEffect(() => {
    const host = hostRef.current
    if (!host) return

    const styles = getComputedStyle(document.documentElement)
    const background = styles.getPropertyValue("--terminal-background").trim()
    const foreground = styles.getPropertyValue("--terminal-foreground").trim()

    const term = new Terminal({
      convertEol: true,
      fontSize: 12,
      scrollback: 2000,
      theme: {
        ...(background ? { background } : {}),
        ...(foreground ? { foreground } : {}),
      },
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(host)
    fit.fit()
    termRef.current = term
    renderedRef.current = []

    // 每帧最多 fit 一次, 并跳过零尺寸: 避免 fit 与重排互相触发造成闪动。
    let frame = 0
    const refit = () => {
      if (frame !== 0) return
      frame = window.requestAnimationFrame(() => {
        frame = 0
        if (host.clientWidth === 0 || host.clientHeight === 0) return
        fit.fit()
      })
    }
    const observer = new ResizeObserver(refit)
    observer.observe(host)

    return () => {
      observer.disconnect()
      if (frame !== 0) window.cancelAnimationFrame(frame)
      term.dispose()
      termRef.current = null
    }
  }, [])

  // 同步显示: 增量追加时只写新行, 否则整体重写 (筛选/时间范围变化)。
  React.useEffect(() => {
    const term = termRef.current
    if (!term) return

    const previous = renderedRef.current
    const isAppend =
      previous.length <= lines.length &&
      previous.every((line, index) => lines[index]?.id === line.id)

    if (!isAppend) {
      term.reset()
    }
    const next = isAppend ? lines.slice(previous.length) : lines
    for (const line of next) {
      term.writeln(formatLine(line))
    }
    renderedRef.current = lines
  }, [lines])

  // 命令: 键入终端并"执行" (dev 阶段只回显结果)
  const runCommand = () => {
    const term = termRef.current
    const value = command.trim()
    setCommand("")
    if (!term || value === "") return
    term.writeln(`\x1b[36m$\x1b[0m ${value}`)
    term.writeln(`${Value("contentArea.logs.commandExecuted") ?? ""}${value}`)
  }

  return (
    <div className={cn("flex min-h-0 flex-col", className)}>
      <div
        ref={hostRef}
        data-slot="log-terminal"
        className="min-h-0 flex-1 overflow-hidden p-2"
      />
      <div className="border-t p-2">
        <RichInput
          prefix={<span className="text-xs">$</span>}
          value={command}
          onChange={(event) => setCommand(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault()
              runCommand()
            }
          }}
          placeholder={Value("contentArea.logs.commandPlaceholder")}
          aria-label={Value("contentArea.logs.commandAria")}
        />
      </div>
    </div>
  )
}
