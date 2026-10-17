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

/** 状态过滤取值: 状态枚举 + all(不过滤)。 */
export type ProxyStatusFilter = ProxyStatus | "all"

/** 新增时的初值(name/target 必填, rules 是 whistle 启动参数的 JSON 文本)。 */
export function emptyInput(): Input {
  return { name: "", target: "", status: "stopped", rules: "{}", description: "" }
}

/** 行 -> 写入入参。 */
export function toInput(row: Proxy): Input {
  return {
    name: row.name,
    target: row.target,
    status: row.status,
    rules: row.rules,
    description: row.description,
  }
}

/** 后端状态字符串 -> 已知状态(未知时退化为 stopped)。 */
export function toStatus(raw: string): ProxyStatus {
  return (PROXY_STATUSES as string[]).includes(raw) ? (raw as ProxyStatus) : "stopped"
}

export type { Input, Proxy }
