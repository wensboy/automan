import { AppWindowIcon, type LucideIcon } from "lucide-react"

/**
 * 代理页的开发期数据: 表格行、状态集合、状态操作与应用列表。
 * 接入真实代理配置后, 页面改为从对应数据源取数即可 (组件不动)。
 */

export type ProxyStatus = "running" | "stopped" | "error" | "exception"

export type ProxyApplication = "chrome" | "edge" | "firefox"

/** 状态操作 (菜单里"切换当前应用状态"的四项)。 */
export type ProxyAction = "start" | "stop" | "restart" | "reload"

export type ProxyRow = {
  id: string
  name: string
  target: string
  status: ProxyStatus
  application: ProxyApplication
  updatedAt: string
  /** web 端点 (抓包页用它内嵌网页); 部分代理没有该端点。 */
  endpoint?: string
}

/** rows-per-page 可选值 (页脚下拉, 不显示提示文本)。 */
export const proxyPageSizes = [5, 10, 20]

export const proxyStatusLabel: Record<ProxyStatus, string> = {
  running: "Running",
  stopped: "Stopped",
  error: "Error",
  exception: "Exception",
}

export const proxyApplicationLabel: Record<ProxyApplication, string> = {
  chrome: "Chrome",
  edge: "Edge",
  firefox: "Firefox",
}

/** 状态操作项: 文案 + 执行后落到哪个状态 (Start/Stop/Restart/Reload 都会切状态)。 */
export const proxyStatusActions: {
  value: ProxyAction
  label: string
  status: ProxyStatus
}[] = [
  { value: "start", label: "Start", status: "running" },
  { value: "stop", label: "Stop", status: "stopped" },
  { value: "restart", label: "Restart", status: "running" },
  { value: "reload", label: "Reload", status: "running" },
]

/** 二级菜单"Application"的选项 (lucide 已移除品牌图标, 统一用 AppWindow 语义图标)。 */
export const proxyApplications: {
  value: ProxyApplication
  label: string
  icon: LucideIcon
}[] = [
  { value: "chrome", label: "Chrome", icon: AppWindowIcon },
  { value: "edge", label: "Edge", icon: AppWindowIcon },
  { value: "firefox", label: "Firefox", icon: AppWindowIcon },
]

export const proxies: ProxyRow[] = [
  { id: "px-1", name: "local-dev", target: "127.0.0.1:8899", status: "running", application: "chrome", updatedAt: "2026-10-13 09:12", endpoint: "http://127.0.0.1:8899" },
  { id: "px-2", name: "stage-api", target: "10.20.4.11:8080", status: "running", application: "edge", updatedAt: "2026-10-13 08:40" },
  { id: "px-3", name: "mock-server", target: "127.0.0.1:3001", status: "stopped", application: "firefox", updatedAt: "2026-10-12 22:05", endpoint: "http://127.0.0.1:3001" },
  { id: "px-4", name: "capture-sink", target: "127.0.0.1:7001", status: "running", application: "chrome", updatedAt: "2026-10-12 21:33", endpoint: "http://127.0.0.1:7001" },
  { id: "px-5", name: "legacy-gateway", target: "192.168.7.20:443", status: "error", application: "edge", updatedAt: "2026-10-12 19:58", endpoint: "https://192.168.7.20" },
  { id: "px-6", name: "upstream-a", target: "api.example.com:443", status: "running", application: "firefox", updatedAt: "2026-10-12 18:20", endpoint: "https://api.example.com" },
  { id: "px-7", name: "upstream-b", target: "api2.example.com:443", status: "running", application: "chrome", updatedAt: "2026-10-12 18:19" },
  { id: "px-8", name: "iot-bridge", target: "10.0.0.42:1883", status: "stopped", application: "edge", updatedAt: "2026-10-12 16:04" },
  { id: "px-9", name: "webrtc-relay", target: "127.0.0.1:3478", status: "running", application: "firefox", updatedAt: "2026-10-12 15:47", endpoint: "http://127.0.0.1:3478" },
  { id: "px-10", name: "grpc-facade", target: "127.0.0.1:50051", status: "exception", application: "chrome", updatedAt: "2026-10-12 14:02" },
  { id: "px-11", name: "socks-lan", target: "192.168.7.9:1080", status: "running", application: "edge", updatedAt: "2026-10-12 11:31", endpoint: "http://192.168.7.9:1080" },
  { id: "px-12", name: "http-lan", target: "192.168.7.9:3128", status: "stopped", application: "firefox", updatedAt: "2026-10-11 17:26" },
  // 以下为凑量数据 (便于观察分页/表体滚动), 接真实数据源时整段删除
  ...Array.from({ length: 28 }, (_, index) => {
    const n = index + 13
    const status = (["running", "stopped", "error", "exception"] as const)[index % 4]
    const application = (["chrome", "edge", "firefox"] as const)[index % 3]
    const host = `10.20.${Math.floor(index / 8) + 5}.${(index % 20) + 10}`
    return {
      id: `px-${n}`,
      name: `service-${n}`,
      target: `${host}:80${index % 9}`,
      status,
      application,
      updatedAt: `2026-10-1${(index % 3) + 1} ${String(9 + (index % 10)).padStart(2, "0")}:${String((index * 7) % 60).padStart(2, "0")}`,
      // 只有"运行中 + 偶数序号"的代理带 web 端点, 便于同时出现两种无效态
      ...(status === "running" && index % 2 === 0 ? { endpoint: `http://${host}:8080` } : {}),
    }
  }),
]

/** 新增时 coder 的初始内容 (dev 占位)。 */
export const proxyConfigTemplate = `{
  "listen": "127.0.0.1:8899",
  "upstream": "api.example.com:443",
  "rules": [
    { "match": "/*", "action": "forward" }
  ]
}`

/** 编辑时 coder 的初始内容: 该代理的全部信息 (代理信息统一在这一个可编辑块内显示)。 */
export function proxyRowConfig(row: ProxyRow): string {
  return JSON.stringify(
    {
      name: row.name,
      target: row.target,
      application: row.application,
      status: row.status,
    },
    null,
    2,
  )
}
