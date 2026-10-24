import { Events } from "@wailsio/runtime"

import { toLogLines, type LogLine, type StreamRecord } from "@/components/custom/logs/model"

/**
 * 日志行缓冲(模块级): 页面切换不再丢历史。
 *
 * 之前的实现把 lines 放在 Logs 页面里, 路由切换会卸载组件 -> 缓冲清空 ->
 * 回到日志页时 xterm 从空开始重建("每次 nav 都重刷")。
 * 现在缓冲与订阅都在模块层: 进入页面只是重新把已有缓冲**重放**到新的 xterm 实例,
 * 只有显式 clear(或超过上限)才丢弃历史。
 */
const MAX_LINES = 500

let lines: LogLine[] = []
let nextId = 0
let subscribed = false
const listeners = new Set<() => void>()

function emit(): void {
  for (const listener of listeners) listener()
}

function append(records: StreamRecord[]): void {
  const appended = toLogLines(records, nextId)
  nextId += appended.length
  const next = [...lines, ...appended]
  lines = next.length > MAX_LINES ? next.slice(next.length - MAX_LINES) : next
  emit()
}

/** 保证只订阅一次后端日志流(幂等; 由页面在挂载后调用)。 */
export function ensureLogSubscription(): void {
  if (subscribed) return
  subscribed = true
  Events.On("log:stream", (event) => {
    const records = event.data as StreamRecord[] | undefined
    if (!Array.isArray(records) || records.length === 0) return
    append(records)
  })
}

/** useSyncExternalStore 的订阅函数。 */
export function subscribeLogs(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** 当前缓冲快照(引用稳定: 只有变更时才换新数组)。 */
export function snapshotLogs(): LogLine[] {
  return lines
}

/** 手动清空(唯一的"丢历史"入口)。 */
export function clearLogs(): void {
  lines = []
  emit()
}
