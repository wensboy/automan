import { createElement, type ReactElement } from "react"
import { Events } from "@wailsio/runtime"
import { toast } from "sonner"

import { Clear, List, Push } from "../../bindings/github.com/wensboy/automan/backend/api/web/noticeservice"
import type { Entry as Notice } from "../../bindings/github.com/wensboy/automan/backend/service/notice/models"

/**
 * 运行时提示(notification)的前端仓储 —— 只有**一条链路**:
 *
 *   notice(...) / 后端 Push
 *        -> service/notice 入队并广播 notice.appended
 *        -> 这里追加队列 + 未读 +1 + toast
 *
 * 因此"投递"永远先发生, "显示"只由事件驱动: 同一入口既覆盖前端触发的提示,
 * 也覆盖后端服务直接投递的提示(不会出现"只在本地 toast、抽屉里没有"的情况)。
 * 队列不持久化; 清除/已读语义见下。
 */
type NoticeState = {
  items: Notice[]
  unread: number
}

let state: NoticeState = { items: [], unread: 0 }
const listeners = new Set<() => void>()

function emit(): void {
  for (const listener of listeners) listener()
}

/** 当前快照(供 useSyncExternalStore 使用; 必须保持引用稳定, 变更时换新对象)。 */
export function snapshotNoticeState(): NoticeState {
  return state
}

/** 订阅变更; 返回退订函数。 */
export function subscribeNotices(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** 与后端队列同步(启动时/打开抽屉时): 以远端为准, 本地未回传的条目按 id 合并保留。 */
export async function loadNotices(): Promise<void> {
  try {
    const remote = (await List()) ?? []
    const remoteIds = new Set(remote.map((item) => item.id))
    const localOnly = state.items.filter((item) => !remoteIds.has(item.id))
    const merged = [...remote, ...localOnly].sort((left, right) => right.at - left.at).slice(0, 200)
    state = { ...state, items: merged }
    emit()
  } catch {
    // 启动早期服务未就绪: 保持现有队列, 后续事件/本地投递仍会追加
  }
}

/** 已弹过 toast 的 id(事件与本地投递可能同时到达同一条, 用它去重)。 */
const toasted = new Set<string>()

/**
 * 投递一条提示: **先入库**。返回落库形态(id 为空表示被勿扰抑制)。
 * 前端业务提示一律走这里, 不要直接调 sonner(否则抽屉里看不到)。
 */
export async function notice(input: {
  level?: "info" | "success" | "warn" | "error"
  label?: string
  title: string
  content?: string
}): Promise<Notice | null> {
  return (await Push(input.level ?? "info", input.label ?? "", input.title, input.content ?? "")) ?? null
}

/**
 * 业务 toast 的统一入口: 先投递 -> 入队 + toast。
 *
 * 以前 toast 只由 notify:append 事件触发, 一旦事件丢失就出现"有 toast、抽屉没有";
 * 现在**本地投递成功即入队并 toast**, 事件到达时按 id 去重(远端触发的提示仍由事件补齐)。
 */
export function notify(
  level: "info" | "success" | "warn" | "error",
  label: string,
  title: string,
  content?: string,
): void {
  void notice({ level, label, title, content })
    .then((entry) => {
      if (entry) upsert(entry, true)
    })
    .catch((error: unknown) => {
      console.error("[notice] push failed:", error)
      toast.error(title, { description: content })
    })
}

/** 清除: 先清后端队列, 再清本地(后端会广播 notify:cleared)。 */
export async function clearNotices(): Promise<void> {
  await Clear()
  toasted.clear()
  state = { items: [], unread: 0 }
  emit()
}

/** 打开抽屉 = 已读: 只归零未读数, 不动队列。 */
export function markNoticesRead(): void {
  if (state.unread === 0) return
  state = { ...state, unread: 0 }
  emit()
}

/** 单条 dismiss: 仅本地移除(后端的环形队列不因单条关闭而删除)。 */
export function dismissNotice(id: string): void {
  state = { ...state, items: state.items.filter((item) => item.id !== id) }
  emit()
}

/**
 * toast 标题: 单行截断, 悬停用 title 属性展示全文。
 * 标题过长时不再横向撑开 toast(容器样式见 share/themed-toaster)。
 */
function toastTitle(text: string): ReactElement {
  return createElement("span", { title: text, className: "block truncate" }, text)
}

function showToast(item: Notice): void {
  const description = item.content || undefined
  const title = toastTitle(item.title)
  if (item.level === "error") {
    toast.error(title, { description })
    return
  }
  if (item.level === "warn") {
    toast.warning(title, { description })
    return
  }
  if (item.level === "success") {
    toast.success(title, { description })
    return
  }
  toast(title, { description })
}

/**
 * 新条目入队(按 id 幂等) + 可选 toast。
 * 本地投递与事件回传都会调它: 同一条只入队一次、只 toast 一次。
 */
function upsert(item: Notice, withToast: boolean): void {
  if (!item || !item.id) return
  const exists = state.items.some((entry) => entry.id === item.id)
  if (!exists) {
    state = { items: [item, ...state.items].slice(0, 200), unread: state.unread + 1 }
    emit()
  }
  if (withToast && !toasted.has(item.id)) {
    if (toasted.size > 512) toasted.clear()
    toasted.add(item.id)
    showToast(item)
  }
}

/** 订阅后端提示事件(应用生命周期内只需调用一次); 返回退订函数。 */
export function attachNoticeEvents(): () => void {
  const offAppend = Events.On("notify:append", (event?: { data?: Notice }) => {
    const payload = event?.data
    if (!payload || !payload.id) return
    upsert(payload, true)
  })
  const offCleared = Events.On("notify:cleared", () => {
    toasted.clear()
    state = { items: [], unread: 0 }
    emit()
  })
  return () => {
    offAppend()
    offCleared()
  }
}
