import { Events } from "@wailsio/runtime"
import { toast } from "sonner"

import { Clear, List } from "../../bindings/github.com/wensboy/automan/backend/api/web/noticeservice"
import type { Notice } from "../../bindings/github.com/wensboy/automan/backend/api/web/models"

/**
 * 运行时提示(notification)的前端仓储:
 *   · 数据源是后端 log 引擎的 notify 队列(内存环形, 不持久化);
 *   · 事件 notify:append 到达时追加 + toast + 未读 +1; notify:cleared 到达时清空(多窗口同步);
 *   · 打开抽屉即视为已读(markNoticesRead), 铃铛红点归零;
 *   · "清除"先调后端 Clear(), 保证前后端一致(不是只清本地副本)。
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

/** 首次拉取后端队列(应用启动时调一次; 失败静默 —— 服务可能尚未就绪)。 */
export async function loadNotices(): Promise<void> {
  try {
    state = { ...state, items: (await List()) ?? [] }
    emit()
  } catch {
    // 启动早期服务未就绪: 保持空队列, 后续事件仍会追加
  }
}

/** 清除: 先清后端队列, 再清本地(后端会广播 notify:cleared)。 */
export async function clearNotices(): Promise<void> {
  await Clear()
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

function pushNotice(notice: Notice): void {
  state = { items: [notice, ...state.items].slice(0, 200), unread: state.unread + 1 }
  emit()

  // 提示即 toast: warn/error 用对应语气, 其余用 success
  if (notice.level === "error") {
    toast.error(notice.title, { description: notice.content || undefined })
  } else if (notice.level === "warn") {
    toast.warning(notice.title, { description: notice.content || undefined })
  } else {
    toast.success(notice.title, { description: notice.content || undefined })
  }
}

/** 订阅后端提示事件(应用生命周期内只需调用一次); 返回退订函数。 */
export function attachNoticeEvents(): () => void {
  const offAppend = Events.On("notify:append", (event?: { data?: Notice }) => {
    const payload = event?.data
    if (!payload || !payload.id) return
    pushNotice(payload)
  })
  const offCleared = Events.On("notify:cleared", () => {
    state = { items: [], unread: 0 }
    emit()
  })
  return () => {
    offAppend()
    offCleared()
  }
}
