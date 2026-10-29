import * as React from "react"
import { cn } from "cn"
import { ActivityIcon, Columns3Icon, HardDriveIcon } from "lucide-react"

import PieBlock from "@/components/custom/share/pie-block"
import EmptyState from "@/components/custom/share/empty-state"
import { useStatic } from "@/components/custom/share/provider/static-provider"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  loadLogDomains,
  loadLogIssues,
  loadLogLevels,
  loadLogStats,
  type LogDomainRow,
  type LogIssueRow,
  type LogLevelSlice,
  type LogStatsView,
} from "@/lib/analysis"
type LogLevelCode = "debug" | "info" | "warn" | "error"

/**
 * Log 分析视图(纯 UI, mock 数据; 见 docs/design/v23.md §69)。
 *
 * 布局与 proxy 相同(h_l->r + v_u->d), 但无 footer:
 *   左: 级别分布(pie, 颜色走静态映射到语义 token)
 *   右上: 领域统计(domain 指日志 `domain: message` 冒号前标识)
 *   右下: 警告/错误日志表(时间范围切换 + 列隐藏, 菜单在时间选择右侧)
 */

type IssueColumn = "id" | "level" | "message" | "extra"

const issueColumnKeys: IssueColumn[] = ["id", "level", "message", "extra"]

function formatClock(ts: number): string {
  const date = new Date(ts)
  const pad = (value: number) => String(value).padStart(2, "0")
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

export default function LogAnalysisView() {
  const { Value } = useStatic()
  const label = (path: string, fallback: string): string => {
    const value = Value(path)
    return typeof value === "string" ? value : fallback
  }
  const text = (key: string, fallback: string): string => label(`contentArea.home.analysis.log.${key}`, fallback)

  const levelLabels = React.useMemo(() => {
    const items = Value("contentArea.logs.levels") ?? []
    return Object.fromEntries(items.map((item) => [String(item.value), String(item.label)])) as Record<string, string>
  }, [Value])

  const ranges = React.useMemo(() => {
    const items = Value("contentArea.home.analysis.log.ranges")
    return Array.isArray(items) ? items : []
  }, [Value])

  const domainColumns = React.useMemo(
    () => ({
      id: text("domainColumns.id", "ID"),
      domain: text("domainColumns.domain", "Domain"),
      total: text("domainColumns.total", "Total"),
      last: text("domainColumns.last", "Last"),
    }),
    [Value],
  )

  const issueColumns = React.useMemo(
    () => ({
      id: text("issueColumns.id", "ID"),
      level: text("issueColumns.level", "Level"),
      message: text("issueColumns.message", "Message"),
      extra: text("issueColumns.extra", "Extra"),
    }),
    [Value],
  )

  const [range, setRange] = React.useState("1h")
  const [loadError, setLoadError] = React.useState("")

  const [levelRows, setLevelRows] = React.useState<LogLevelSlice[]>([])
  const [domainRows, setDomainRows] = React.useState<LogDomainRow[]>([])
  const [issueRows, setIssueRows] = React.useState<LogIssueRow[]>([])
  const [visible, setVisible] = React.useState<Record<IssueColumn, boolean>>({
    id: true,
    level: true,
    message: true,
    extra: true,
  })

  // 停留期间每 10s 轮询一次; 离开视图即停止(useEffect 清理)
  React.useEffect(() => {
    let alive = true
    const pull = async () => {
      try {
        const [levels, domains, issues] = await Promise.all([
          loadLogLevels(),
          loadLogDomains(100),
          loadLogIssues(range, 200),
        ])
        if (!alive) return
        setLevelRows(levels)
        setDomainRows(domains)
        setIssueRows(issues)
      } catch (error) {
        if (alive) setLoadError(error instanceof Error ? error.message : String(error))
      }
    }
    void pull()
    const timer = window.setInterval(() => void pull(), 10_000)
    return () => {
      alive = false
      window.clearInterval(timer)
    }
  }, [range])


  const levelColors: Record<string, string> = {
    debug: "var(--muted-foreground)",
    info: "var(--primary)",
    warn: "var(--status-stopped)",
    error: "var(--status-error)",
  }

  const levelSlices = React.useMemo(
    () =>
      levelRows.map((row) => ({
        name: levelLabels[row.level] ?? row.level,
        value: Number(row.value),
        fill: levelColors[row.level] ?? "var(--primary)",
      })),
    [levelRows, levelLabels],
  )

  const issues = React.useMemo(() => {
    // 后端已按范围过滤; mock 回落时前端自行过滤
    return issueRows
  }, [issueRows])

  const columns = issueColumnKeys.filter((key) => visible[key])

  return (
    <div className="flex min-h-0 flex-1 gap-2 p-2">
      {/* 左: 级别分布 */}
      <Card className="flex min-h-0 min-w-0 flex-[1] flex-col gap-0 py-0">
        <CardHeader className="px-3 py-2">
          <CardTitle className="text-sm font-medium">{text("levelTitle", "Level distribution")}</CardTitle>
        </CardHeader>
        <CardContent className="flex min-h-0 flex-1 flex-col px-2 pb-2">
          {levelRows.length === 0 ? (
            <div className="flex min-h-0 flex-1 items-center justify-center">
            <EmptyState
              title={text("levelTitle", "Level distribution")}
              description={loadError || text("empty", "No log records yet.")}
            />
            </div>
          ) : (
            <PieBlock data={levelSlices} />
          )}
        </CardContent>
      </Card>

      {/* 右: 领域统计 + 警告错误 */}
      <div className="flex min-h-0 min-w-0 flex-[2] flex-col gap-2">
        <Card className="flex min-h-0 flex-[2] flex-col gap-0 py-0">
          <CardHeader className="px-3 py-2">
            <CardTitle className="text-sm font-medium">{text("domainTitle", "Domain statistics")}</CardTitle>
          </CardHeader>
          <CardContent className="min-h-0 flex-1 overflow-auto px-0 pb-0">
            <table className="w-full min-w-full caption-bottom text-xs">
              <thead className="sticky top-0 z-10 bg-background">
                <tr className="border-b text-left text-muted-foreground">
                  <th className="whitespace-nowrap px-3 py-1.5 font-medium">{domainColumns.id}</th>
                  <th className="whitespace-nowrap px-3 py-1.5 font-medium">{domainColumns.domain}</th>
                  <th className="whitespace-nowrap px-3 py-1.5 font-medium">{domainColumns.total}</th>
                  <th className="whitespace-nowrap px-3 py-1.5 font-medium">{domainColumns.last}</th>
                </tr>
              </thead>
              <tbody>
                {domainRows.map((row) => (
                  <tr key={row.id} className="border-b border-border/50 last:border-0 hover:bg-muted/40">
                    <td className="max-w-[80px] truncate whitespace-nowrap px-3 py-1.5" title={String(row.id)}>
                      {row.id}
                    </td>
                    <td className="max-w-[180px] truncate whitespace-nowrap px-3 py-1.5" title={row.domain}>
                      {row.domain}
                    </td>
                    <td className="max-w-[100px] truncate whitespace-nowrap px-3 py-1.5 tabular-nums" title={String(row.total)}>
                      {row.total}
                    </td>
                    <td
                      className="max-w-[120px] truncate whitespace-nowrap px-3 py-1.5 tabular-nums text-muted-foreground"
                      title={formatClock(row.last)}
                    >
                      {formatClock(row.last)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>

        <Card className="flex min-h-0 flex-[3] flex-col gap-0 py-0">
          <CardHeader className="flex flex-row items-center justify-between gap-2 px-3 py-2">
            <CardTitle className="text-sm font-medium">{text("issueTitle", "Warnings & errors")}</CardTitle>
            <div className="flex items-center gap-1">
              <Select value={range} onValueChange={setRange}>
                <SelectTrigger size="sm" className="h-7 w-20" aria-label={text("timeAria", "Time range")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent position="popper" align="end">
                  {ranges.map((item) => (
                    <SelectItem key={String(item.value)} value={String(item.value)}>
                      {String(item.label)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-6"
                    aria-label={text("toggleColumnsAria", "Toggle columns")}
                  >
                    <Columns3Icon className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-40">
                  <DropdownMenuLabel>{text("columnsTitle", "Columns")}</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  {issueColumnKeys.map((key) => (
                    <DropdownMenuCheckboxItem
                      key={key}
                      checked={visible[key]}
                      onCheckedChange={(checked) => setVisible((prev) => ({ ...prev, [key]: Boolean(checked) }))}
                      onSelect={(event) => event.preventDefault()}
                    >
                      {issueColumns[key]}
                    </DropdownMenuCheckboxItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </CardHeader>
          <CardContent className="min-h-0 flex-1 overflow-auto px-0 pb-0">
            <table className="w-full min-w-full caption-bottom text-xs">
              <thead className="sticky top-0 z-10 bg-background">
                <tr className="border-b text-left text-muted-foreground">
                  {columns.map((key) => (
                    <th key={key} className="whitespace-nowrap px-3 py-1.5 font-medium">
                      {issueColumns[key]}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {issues.map((row) => (
                  <tr key={row.id} className="border-b border-border/50 last:border-0 hover:bg-muted/40">
                    {columns.map((key) => {
                      const value =
                        key === "id"
                          ? row.id
                          : key === "level"
                            ? (levelLabels[row.level as LogLevelCode] ?? row.level)
                            : key === "message"
                              ? row.message
                              : row.extra
                      return (
                        <td
                          key={key}
                          className={cn(
                            "max-w-[260px] truncate whitespace-nowrap px-3 py-1.5",
                            key === "id" && "max-w-[80px] tabular-nums",
                            key === "level" && "max-w-[90px]",
                          )}
                          title={String(value)}
                        >
                          {value}
                        </td>
                      )
                    })}
                  </tr>
                ))}
                {issues.length === 0 && (
                  <tr>
                    <td className="px-3 py-6 text-center text-muted-foreground" colSpan={Math.max(1, columns.length)}>
                      -
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

/** footer 左侧: 日志缓冲大小与累计总数(图标 badge, 数据 mock)。 */
export function LogFooterInfo() {
  const { Value } = useStatic()
  const [stats, setStats] = React.useState<LogStatsView>({ total: 0, buffer: 0 })

  React.useEffect(() => {
    let alive = true
    const pull = async () => {
      try {
        const next = await loadLogStats()
        if (alive) setStats(next)
      } catch {
        // 保持上一次
      }
    }
    void pull()
    const timer = window.setInterval(() => void pull(), 10_000)
    return () => {
      alive = false
      window.clearInterval(timer)
    }
  }, [])
  const label = (path: string, fallback: string): string => {
    const value = Value(path)
    return typeof value === "string" ? value : fallback
  }

  return (
    <div className="flex items-center gap-1">
      <Badge
        variant="secondary"
        className="gap-1 text-xs font-normal"
        aria-label={`${label("contentArea.home.analysis.log.bufferAria", "Log buffer size")}: ${stats.buffer}`}
      >
        <HardDriveIcon className="size-3.5" />
        <span>{label("contentArea.home.analysis.log.bufferLabel", "Buffer")}</span>
        <span className="font-medium tabular-nums text-foreground">{stats.buffer}</span>
      </Badge>
      <Badge
        variant="secondary"
        className="gap-1 text-xs font-normal"
        aria-label={`${label("contentArea.home.analysis.log.totalAria", "Total logs")}: ${stats.total}`}
      >
        <ActivityIcon className="size-3.5" />
        <span>{label("contentArea.home.analysis.log.totalLabel", "Total")}</span>
        <span className="font-medium tabular-nums text-foreground">{stats.total.toLocaleString("en-US")}</span>
      </Badge>
    </div>
  )
}
