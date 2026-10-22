import type {
  Input,
  Proxy,
} from "../../../../bindings/github.com/wensboy/automan/backend/service/proxy"

/**
 * 代理页的领域模型: 行/入参类型来自生成的 bindings, 这里只放 UI 侧的换算与常量。
 * (对应后端 service/proxy, 见 docs/design/v17.md)
 */

/** 代理状态: 与后端 CHECK 约束一致(不含过程态)。 */
export type ProxyStatus = "running" | "stopped" | "error" | "exception"

export const PROXY_STATUSES: ProxyStatus[] = ["running", "stopped", "error", "exception"]

/** 行菜单的状态动作: start/stop/restart/reload(restart = 先停再起)。 */
export type ProxyAction = "start" | "stop" | "restart" | "reload"

/** 行菜单里的动作项(文案由 static 数据提供, 这里只给顺序与取值)。 */
export const proxyStatusActions: { value: ProxyAction }[] = [
  { value: "start" },
  { value: "stop" },
  { value: "restart" },
  { value: "reload" },
]

/** 状态过滤取值: 状态枚举 + all(不过滤)。 */
export type ProxyStatusFilter = ProxyStatus | "all"

/**
 * 新增时的初值: name 必填; **target 不再是入参**(由 rules 的 host/port 派生, 后端落库冗余存储),
 * rules 是 whistle 启动参数的 JSON 文本。
 */
export function emptyInput(): Input {
  return { name: "", target: "", rules: "{}", description: "" }
}

/**
 * 从 rules 派生 target = host:port(与后端同一口径)。
 * 只用于**展示**(编辑器里让用户看到最终会写成什么); 落库值以服务端派生为准。
 */
export function targetOf(rules: string): string {
  try {
    const parsed = JSON.parse(rules) as { host?: unknown; port?: unknown }
    const host = typeof parsed.host === "string" ? parsed.host.trim() : ""
    if (!host) return ""
    const port = typeof parsed.port === "number" ? parsed.port : Number.parseInt(String(parsed.port ?? ""), 10)
    return Number.isFinite(port) && port > 0 ? `${host}:${port}` : host
  } catch {
    return ""
  }
}

/** 行 -> 写入入参(rules 以 pretty 形式进编辑器, 便于阅读)。 */
export function toInput(row: Proxy): Input {
  return {
    name: row.name,
    target: row.target,
    rules: prettyJSON(row.rules),
    description: row.description,
  }
}

/**
 * 把 JSON 文本压成**紧凑字符串**(入库形态: 不含缩进与换行)。
 * 解析失败原样返回, 交给后端给出明确的 JSON 校验错误(前端不替用户"修正"数据)。
 */
export function compactJSON(raw: string): string {
  const trimmed = (raw ?? "").trim()
  if (trimmed === "") return raw ?? ""
  try {
    return JSON.stringify(JSON.parse(trimmed))
  } catch {
    return raw
  }
}

/** 写入入参归一: rules 紧凑落库; target 不传(服务端按 rules 派生, 避免前端/后端两套口径)。 */
export function toPayload(input: Input): Input {
  return { ...input, target: "", rules: compactJSON(input.rules) }
}

/**
 * 把 JSON 文本格式化为 2 空格缩进(供编辑器显示)。
 * 解析失败时**原样返回** —— 非法内容是用户正在编辑的中间态, 不能悄悄改掉或清空。
 */
export function prettyJSON(raw: string): string {
  const trimmed = (raw ?? "").trim()
  if (trimmed === "") return raw ?? ""
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return raw
  }
}

/** 后端状态字符串 -> 已知状态(未知时退化为 stopped)。 */
export function toStatus(raw: string): ProxyStatus {
  return (PROXY_STATUSES as string[]).includes(raw) ? (raw as ProxyStatus) : "stopped"
}

export type { Input, Proxy }
