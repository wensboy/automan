import {
  LogDomains,
  LogIssues,
  LogLevels,
  LogStats,
  ProxyActions,
  ProxyStatuses,
  ProxyTopK,
  ProxyTotal,
  Workspaces,
  ServiceSnapshot,
  SystemInfo,
} from "../../bindings/github.com/wensboy/automan/backend/api/web/analysisservice"

/**
 * Analysis 数据访问层(见 docs/design/v25.md §9)。
 *
 * 约定:
 *   · 只做"后端 -> 前端视图形态"的适配; 失败抛错, 由视图决定是否回落 mock;
 *   · 所有结构以 mock 的字段名为准, 保证换数据源时组件不动。
 */
export type ProxyStatusSlice = { status: string; value: number }
export type ProxyActionRow = { id: number; name: string; workspace: string; listen: string; action: string; at: number }
export type WorkspaceOption = { id: string; name: string }
export type ProxyTopkRow = { name: string; count: number }

export type LogLevelSlice = { level: string; value: number }
export type LogDomainRow = { id: number; domain: string; total: number; last: number }
export type LogIssueRow = { id: number; level: "warn" | "error"; message: string; extra: string; ts: number }
export type LogStatsView = { total: number; buffer: number }
export type SystemRow = { id: number; key: string; value: string }

export async function loadProxyStatuses(workspaceId = ""): Promise<ProxyStatusSlice[]> {
  const rows = (await ProxyStatuses(workspaceId)) ?? []
  return rows.map((row) => ({ status: String(row.status), value: Number(row.count) }))
}

export async function loadProxyActions(limit = 200): Promise<ProxyActionRow[]> {
  const rows = (await ProxyActions(limit)) ?? []
  return rows.map((row) => ({
    id: Number(row.id),
    name: String(row.name),
    workspace: String(row.workspace ?? ""),
    listen: String(row.listen),
    action: String(row.action),
    at: Number(row.at),
  }))
}

export async function loadProxyTotal(): Promise<number> {
  return Number(await ProxyTotal())
}

export async function loadWorkspaces(): Promise<WorkspaceOption[]> {
  const rows = (await Workspaces()) ?? []
  return rows.map((row) => ({ id: String(row.id), name: String(row.name) }))
}

export async function loadProxyTopK(k: number): Promise<ProxyTopkRow[]> {
  const rows = (await ProxyTopK(k)) ?? []
  return rows.map((row) => ({ name: String(row.name), count: Number(row.count) }))
}

export async function loadLogLevels(): Promise<LogLevelSlice[]> {
  const rows = (await LogLevels()) ?? []
  return rows.map((row) => ({ level: String(row.level), value: Number(row.count) }))
}

export async function loadLogDomains(limit = 100): Promise<LogDomainRow[]> {
  const rows = (await LogDomains(limit)) ?? []
  return rows.map((row) => ({
    id: Number(row.id),
    domain: String(row.domain),
    total: Number(row.total),
    last: Number(row.last),
  }))
}

export async function loadLogIssues(rangeKey: string, limit = 200): Promise<LogIssueRow[]> {
  const rows = (await LogIssues(rangeKey, limit)) ?? []
  return rows.map((row) => ({
    id: Number(row.id),
    level: String(row.level) === "error" ? "error" : "warn",
    message: String(row.message),
    extra: String(row.extra),
    ts: Number(row.at),
  }))
}

export async function loadLogStats(): Promise<LogStatsView> {
  const stats = await LogStats()
  return { total: Number(stats?.total ?? 0), buffer: Number(stats?.buffer ?? 0) }
}

export async function loadSystemInfo(): Promise<SystemRow[]> {
  const rows = (await SystemInfo()) ?? []
  return rows.map((row) => ({ id: Number(row.id), key: String(row.key), value: String(row.value) }))
}

export async function loadServiceSnapshot(bridgeService: string) {
  return ServiceSnapshot(bridgeService)
}

