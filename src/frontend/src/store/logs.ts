import { create } from "zustand"

import { Recent } from "../../bindings/github.com/wensboy/automan/backend/api/web/logservice"

import { toLogLines, type LogLine, type StreamRecord } from "@/components/custom/logs/model"

/**
 * 日志 store(app 级): 拉取与缓冲都在 store 里, 页面只订阅。
 *
 * 为什么放进 store(见 docs/design/v23.md §52):
 *   之前缓冲/轮询挂在日志页组件的生命周期上, 页面没加载时没有任何人拉取,
 *   且"组件挂载时机"会影响第一次全量/增量的判定 => UI 看起来不跟随日志变化。
 *   现在: 应用启动即开始轮询, 页面只做渲染; 组件何时挂载都不影响数据。
 *
 * 拉取策略:
 *   · 进入/启动: 取一批元信息 -> 小队列/首次/游标被裁 => 全量(1000), 否则只拉增量
 *   · 常驻: 400ms 增量轮询
 *   · clear: 记 floor(清除点), 之后全量也只从该点往后, 已清除的不复活
 */
const MAX_LINES = 500
const POLL_MS = 400
const PAGE_SIZE = 200
const SMALL_QUEUE = 200
const FULL_LIMIT = 1000

type LogsState = {
  /** 已渲染的日志行(最新在末)。 */
  lines: LogLine[]
  /** 已消费到的 seq 游标。 */
  cursor: number
  /** 清除点: 全量拉取的起点。 */
  floor: number
  /** 手动清空(唯一的"丢历史"入口)。 */
  clear: () => void
}

export const useLogsStore = create<LogsState>((set, get) => ({
  lines: [],
  cursor: 0,
  floor: 0,
  clear: () => {
    const { cursor } = get()
    set({ lines: [], floor: cursor })
  },
}))

// 以下为"非渲染态"的运行时句柄: 放模块级, 避免进 store 造成无意义的重渲染
let nextId = 0
let timer: number | null = null
let inFlight = false

function emit(records: StreamRecord[], replaceBefore: boolean): void {
  if (replaceBefore) {
    nextId = 0
  }
  const appended = records.length > 0 ? toLogLines(records, nextId) : []
  nextId += appended.length
  const { lines } = useLogsStore.getState()
  const base = replaceBefore ? [] : lines
  const next = [...base, ...appended]
  useLogsStore.setState({
    lines: next.length > MAX_LINES ? next.slice(next.length - MAX_LINES) : next,
  })
}

/** 进入/启动时: 按队列规模选择全量或增量。 */
async function bootstrap(): Promise<void> {
  const { cursor, floor } = useLogsStore.getState()
  try {
    const batch = await Recent(cursor, PAGE_SIZE)
    if (!batch) return
    const total = batch.total ?? 0
    const oldest = batch.oldest ?? 0
    const expired = cursor !== 0 && oldest > 0 && cursor < oldest

    if (cursor === 0 || expired || total <= SMALL_QUEUE) {
      const full = await Recent(floor, FULL_LIMIT)
      emit((full?.items ?? []) as unknown as StreamRecord[], true)
      useLogsStore.setState({ cursor: full?.seq ?? cursor })
      return
    }
    emit((batch.items ?? []) as unknown as StreamRecord[], false)
    useLogsStore.setState({ cursor: batch.seq ?? cursor })
  } catch {
    // 服务未就绪/短暂失败: 交给轮询重试
  }
}

/** 增量拉取。 */
async function tick(): Promise<void> {
  if (inFlight) return
  inFlight = true
  try {
    const { cursor } = useLogsStore.getState()
    const batch = await Recent(cursor, PAGE_SIZE)
    if (batch) {
      emit((batch.items ?? []) as unknown as StreamRecord[], false)
      useLogsStore.setState({ cursor: batch.seq ?? cursor })
    }
  } catch {
    // 静默: 下一周期重试
  } finally {
    inFlight = false
  }
}

/**
 * 手动刷新: 以清除点(floor)为起点做一次全量替换, 并把游标推到最新。
 * 用途: 前端的增量状态万一与后端不一致(例如极端时序), 一次刷新即可对齐。
 */
export async function refreshLogs(): Promise<void> {
  const { floor } = useLogsStore.getState()
  try {
    const full = await Recent(floor, FULL_LIMIT)
    emit((full?.items ?? []) as unknown as StreamRecord[], true)
    useLogsStore.setState({ cursor: full?.seq ?? 0 })
  } catch {
    // 失败保持现状, 由轮询继续
  }
}

/** 应用启动时调用一次(幂等): 常驻轮询, 与页面是否打开无关。 */
export function ensureLogPolling(): void {
  if (timer !== null) return
  void bootstrap().then(() => tick())
  timer = window.setInterval(() => void tick(), POLL_MS)
}
