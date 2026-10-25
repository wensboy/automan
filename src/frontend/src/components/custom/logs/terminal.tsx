import * as React from "react"
import { FitAddon } from "@xterm/addon-fit"
import { Terminal } from "@xterm/xterm"
import { cn } from "cn"

import RichInput from "@/components/custom/share/rich-input"

import type { LogLine } from "@/components/custom/logs/model"
import { formatDateTime } from "@/lib/format"

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
  // 含年月日 (本地时区), 统一由 lib/format.ts 的 formatDateTime 提供
  const time = formatDateTime(line.time, { seconds: true })
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
  // 最新行(创建终端的瞬间可能还没轮到写入 effect, 用它做一次权威写入)
  // 是否"贴底"跟随: 用户主动上滚后不再强制拉回底部
  const stickRef = React.useRef(true)
  const linesRef = React.useRef<LogLine[]>(lines)
  linesRef.current = lines
  const { Value } = useStatic()
  const [command, setCommand] = React.useState("")

  // ready 用于"等终端真正建好再写行": 首次进入页面时容器可能还没有尺寸,
  // 若此时 open() 会得到 0 行列, 后续即使有数据也看不到(表现为"第二次进才显示")。
  const [ready, setReady] = React.useState(false)

  // 初始化终端 (挂载一次); 主题色取 token —— xterm 不解析 oklch, 故 token 用可解析格式。
  React.useEffect(() => {
    const host = hostRef.current
    if (!host) return

    let term: Terminal | null = null
    let observer: ResizeObserver | null = null
    let frame = 0

    const refit = () => {
      if (!term || frame !== 0) return
      frame = window.requestAnimationFrame(() => {
        frame = 0
        if (host.clientWidth === 0 || host.clientHeight === 0) return
      })
    }

    const create = () => {
      const styles = getComputedStyle(document.documentElement)
      const background = styles.getPropertyValue("--terminal-background").trim()
      const foreground = styles.getPropertyValue("--terminal-foreground").trim()

      const instance = new Terminal({
        convertEol: true,
        fontSize: 12,
        // 与界面一致的内置字体: xterm 默认用 Courier 系, 不同机器度量不同会串行
        fontFamily: '"JetBrains Mono Variable", ui-monospace, SFMono-Regular, monospace',
        scrollback: 2000,
        theme: {
          ...(background ? { background } : {}),
          ...(foreground ? { foreground } : {}),
        },
      })
      const fit = new FitAddon()
      instance.loadAddon(fit)
      instance.open(host)
      // xterm 5.5 + addon-fit 0.11: fit() 可用(升到 xterm 6 会与 0.11 不兼容, 故固定 5.5)
      fit.fit()

      // 记录用户是否还在底部(上滚查看历史时不要强行拉回)
      instance.onScroll(() => {
        const active = instance.buffer.active
        stickRef.current = active.baseY - active.viewportY <= 0
      })

      term = instance
      termRef.current = instance
      renderedRef.current = []

      // 关键: 创建完成即刻写入当前缓冲 —— 首次进入页面时数据往往已经在 store 里,
      // 只靠 effect 依赖 ready 的变化存在时序竞争(实测会首次进入空白, 第二次才有)。
      const current = linesRef.current
      if (current.length > 0) {
        for (const line of current) instance.writeln(formatLine(line))
        stickRef.current = true
        instance.scrollToBottom()
        renderedRef.current = current
      }

      // 再补一次: 首次进入时布局/字体可能刚稳定
      window.requestAnimationFrame(() => {
        if (host.clientWidth === 0 || host.clientHeight === 0) return
        fit.fit()
        instance.scrollToBottom()
      })

      setReady(true)

      observer = new ResizeObserver(refit)
      observer.observe(host)
      // 内置字体异步加载: 就绪后再 fit 一次, 避免按后备字体算列宽
      void document.fonts?.ready.then(() => refit()).catch(() => undefined)
    }

    const boot = () => {
      // 容器有尺寸再 open: 首次进入时布局可能还没跑完
      if (host.clientWidth > 0 && host.clientHeight > 0) {
        create()
        return
      }
      const wait = new ResizeObserver(() => {
        if (host.clientWidth === 0 || host.clientHeight === 0) return
        wait.disconnect()
        create()
      })
      wait.observe(host)
      observer = wait
    }

    frame = window.requestAnimationFrame(boot)

    return () => {
      if (frame !== 0) window.cancelAnimationFrame(frame)
      observer?.disconnect()
      term?.dispose()
      termRef.current = null
      setReady(false)
    }
  }, [])

  // 同步显示: 增量追加时只写新行, 否则整体重写 (筛选/时间范围变化)。
  React.useEffect(() => {
    const term = termRef.current
    if (!term || !ready) return

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
    // 贴底时才自动滚动: 用户上滚查看历史时不打扰
    if (next.length > 0 && stickRef.current) term.scrollToBottom()
    renderedRef.current = lines
  }, [lines, ready])

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
