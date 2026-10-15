/**
 * 记录页的开发期数据。
 * 刻意与 mock/proxies.ts 保持独立(不做共享): 记录页与代理页后续会各自演进。
 */

export type RecordStatus = "running" | "stopped" | "error" | "exception"

export type RecordRow = {
  id: string
  name: string
  target: string
  status: RecordStatus
  updatedAt: string
}

/** rows-per-page 可选值 (页脚下拉, 不显示提示文本)。 */
export const recordPageSizes = [5, 10, 20]

export const recordStatusLabel: Record<RecordStatus, string> = {
  running: "Running",
  stopped: "Stopped",
  error: "Error",
  exception: "Exception",
}

export const records: RecordRow[] = [
  { id: "rc-1", name: "session-0912", target: "127.0.0.1:8899", status: "running", updatedAt: "2026-10-13 09:12" },
  { id: "rc-2", name: "session-0840", target: "10.20.4.11:8080", status: "running", updatedAt: "2026-10-13 08:40" },
  { id: "rc-3", name: "session-2205", target: "127.0.0.1:3001", status: "stopped", updatedAt: "2026-10-12 22:05" },
  { id: "rc-4", name: "session-2133", target: "127.0.0.1:7001", status: "running", updatedAt: "2026-10-12 21:33" },
  { id: "rc-5", name: "session-1958", target: "192.168.7.20:443", status: "error", updatedAt: "2026-10-12 19:58" },
  { id: "rc-6", name: "session-1820", target: "api.example.com:443", status: "running", updatedAt: "2026-10-12 18:20" },
  { id: "rc-7", name: "session-1819", target: "api2.example.com:443", status: "running", updatedAt: "2026-10-12 18:19" },
  { id: "rc-8", name: "session-1604", target: "10.0.0.42:1883", status: "stopped", updatedAt: "2026-10-12 16:04" },
  { id: "rc-9", name: "session-1547", target: "127.0.0.1:3478", status: "running", updatedAt: "2026-10-12 15:47" },
  { id: "rc-10", name: "session-1402", target: "127.0.0.1:50051", status: "exception", updatedAt: "2026-10-12 14:02" },
  { id: "rc-11", name: "session-1131", target: "192.168.7.9:1080", status: "running", updatedAt: "2026-10-12 11:31" },
  { id: "rc-12", name: "session-1012", target: "192.168.7.9:3128", status: "stopped", updatedAt: "2026-10-11 17:26" },
  // 以下为凑量数据 (便于观察分页/表体滚动), 接真实数据源时整段删除
  ...Array.from({ length: 28 }, (_, index) => {
    const n = index + 13
    const status = (["running", "stopped", "error", "exception"] as const)[index % 4]
    const host = `10.30.${Math.floor(index / 8) + 5}.${(index % 20) + 10}`
    return {
      id: `rc-${n}`,
      name: `session-${String(n).padStart(4, "0")}`,
      target: `${host}:81${index % 9}`,
      status,
      updatedAt: `2026-10-1${(index % 3) + 1} ${String(9 + (index % 10)).padStart(2, "0")}:${String((index * 11) % 60).padStart(2, "0")}`,
    }
  }),
]

/** 编辑时 coder 的初始内容 (dev 占位): 该记录的全部信息。 */
export function recordRowContent(row: RecordRow): string {
  return JSON.stringify(
    {
      name: row.name,
      target: row.target,
      status: row.status,
      updatedAt: row.updatedAt,
    },
    null,
    2,
  )
}
