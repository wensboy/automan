import * as React from "react"
import { cn } from "cn"
import { Columns3Icon, NetworkIcon } from "lucide-react"
import PieBlock from "@/components/custom/share/pie-block"
import EmptyState from "@/components/custom/share/empty-state"
import { useStatic } from "@/components/custom/share/provider/static-provider"
import {
  loadProxyActions,
  loadProxyStatuses,
  loadProxyTopK,
  loadProxyTotal,
  loadWorkspaces,
  type WorkspaceOption,
  type ProxyActionRow,
  type ProxyStatusSlice,
  type ProxyTopkRow,
} from "@/lib/analysis"
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"

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

/**
 * Proxy 分析视图(纯 UI, mock 数据; 见 docs/design/v23.md §61)。
 *
 * 布局: h_l->r( 状态分布 pie, v_u->d( 操作追踪 table, topk 频率 bar ) )
 * 约定:
 *   - 左右两列等宽; 每个块独立 Card, 块间保留小 gap;
 *   - pie 颜色与代理状态语义色一致(--status-*);
 *   - 表格 x/y 都可滚动, 列可隐藏(右上菜单);
 *   - bar 按频率降序, k 可切换。
 */

type ColumnKey = "id" | "name" | "workspace" | "listen" | "action"

const columnKeys: ColumnKey[] = ["id", "name", "workspace", "listen", "action"]

export default function ProxyAnalysisView() {
  const { Value } = useStatic()
  const label = (path: string, fallback: string): string => {
    const value = Value(path)
    return typeof value === "string" ? value : fallback
  }
  const text = (key: string, fallback: string): string => label(`contentArea.home.analysis.proxy.${key}`, fallback)

  const columnLabels: Record<ColumnKey, string> = {
    id: text("columns.id", "ID"),
    name: text("columns.name", "Name"),
    workspace: text("columns.workspace", "Workspace"),
    listen: text("columns.listen", "Listen"),
    action: text("columns.action", "Action"),
  }

  const [visible, setVisible] = React.useState<Record<ColumnKey, boolean>>({
    id: true,
    name: true,
    workspace: true,
    listen: true,
    action: true,
  })
  const [topk, setTopk] = React.useState(5)

  // 真实数据(失败回落 mock, 保证视图可用)
  const [statusRows, setStatusRows] = React.useState<ProxyStatusSlice[]>([])
  const [actions, setActions] = React.useState<ProxyActionRow[]>([])
  const [topkRows, setTopkRows] = React.useState<ProxyTopkRow[]>([])

  const [workspaces, setWorkspaces] = React.useState<WorkspaceOption[]>([])
  const [workspace, setWorkspace] = React.useState("")
  const [loadError, setLoadError] = React.useState("")

  React.useEffect(() => {
    let alive = true
    void (async () => {
      try {
        const options = await loadWorkspaces()
        if (alive) setWorkspaces(options)
      } catch {
        // 无工作区时下拉为空
      }
    })()
    return () => {
      alive = false
    }
  }, [])

  React.useEffect(() => {
    let alive = true
    void (async () => {
      try {
        const [statuses, actionRows] = await Promise.all([loadProxyStatuses(workspace), loadProxyActions(200)])
        if (!alive) return
        setStatusRows(statuses)
        setActions(actionRows)
      } catch (error) {
        if (alive) setLoadError(error instanceof Error ? error.message : String(error))
      }
    })()
    return () => {
      alive = false
    }
  }, [workspace])

  React.useEffect(() => {
    let alive = true
    void (async () => {
      try {
        const rows = await loadProxyTopK(topk)
        if (alive && rows.length > 0) setTopkRows(rows)
      } catch {
        // 保持 mock
      }
    })()
    return () => {
      alive = false
    }
  }, [topk])

  const kChoices = React.useMemo(() => {
    const items = Value("contentArea.home.analysis.proxy.kOptions")
    return Array.isArray(items) ? items.map((item) => Number(item)).filter((n) => n > 0) : [5, 10, 15, 20]
  }, [Value])

  const statusSlices = React.useMemo(
    () =>
      statusRows.map((row) => ({
        name: label(`contentArea.proxy.statusLabel.${row.status}`, row.status),
        value: Number(row.value),
        fill: `var(--status-${row.status})`,
      })),
    [statusRows, Value],
  )

  const columns = columnKeys.filter((key) => visible[key])
  const topkSlice = React.useMemo(() => topkRows.slice(0, topk), [topkRows, topk])


  return (
    <div className="flex min-h-0 flex-1 gap-2 p-2">
      {/* 左: 状态分布 */}
      <Card className="flex min-h-0 flex-1 flex-col gap-0 py-0">
        <CardHeader className="flex flex-row items-center justify-between gap-2 px-3 py-2">
          <CardTitle className="text-sm font-medium">{text("statusTitle", "Status distribution")}</CardTitle>
          <Select value={workspace || "__all__"} onValueChange={(value) => setWorkspace(value === "__all__" ? "" : value)}>
            <SelectTrigger size="sm" className="h-7 w-32" aria-label={text("workspaceAria", "Workspace")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent position="popper" align="end">
              <SelectItem value="__all__">{text("workspaceAll", "All workspaces")}</SelectItem>
              {workspaces.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {item.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </CardHeader>
        <CardContent className="flex min-h-0 flex-1 flex-col px-2 pb-2">
          {statusRows.length === 0 ? (
            <div className="flex min-h-0 flex-1 items-center justify-center">
            <EmptyState
              title={text("statusTitle", "Status distribution")}
              description={loadError || text("empty", "No proxy data yet.")}
            />
            </div>
          ) : (
            <PieBlock data={statusSlices} />
          )}
        </CardContent>
      </Card>

      {/* 右: 操作追踪 + topk */}
      <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-2">
        <Card className="flex min-h-0 flex-[3] flex-col gap-0 py-0">
          <CardHeader className="flex flex-row items-center justify-between gap-2 px-3 py-2">
            <CardTitle className="text-sm font-medium">{text("traceTitle", "Operation trace")}</CardTitle>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="size-6" aria-label={text("toggleColumnsAria", "Toggle columns")}>
                  <Columns3Icon className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-40">
                <DropdownMenuLabel>{text("columnsTitle", "Columns")}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                {columnKeys.map((key) => (
                  <DropdownMenuCheckboxItem
                    key={key}
                    checked={visible[key]}
                    onCheckedChange={(checked) => setVisible((prev) => ({ ...prev, [key]: Boolean(checked) }))}
                    onSelect={(event) => event.preventDefault()}
                  >
                    {columnLabels[key]}
                  </DropdownMenuCheckboxItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </CardHeader>
          <CardContent
            className={cn(
              "min-h-0 flex-1 overflow-auto px-0 pb-0",
              // 滚动条一律零宽(不占布局): 避免悬停时出现导致右侧 gap/割裂
              "[&::-webkit-scrollbar]:h-0 [&::-webkit-scrollbar]:w-0",
              "[&::-webkit-scrollbar-thumb]:bg-transparent",
              "[&::-webkit-scrollbar-track]:bg-transparent",
            )}
          >
            <table className="w-full min-w-full caption-bottom text-xs">
              <thead className="sticky top-0 z-10 bg-background">
                <tr className="border-b text-left text-muted-foreground">
                  {columns.map((key) => (
                    <th key={key} className="whitespace-nowrap px-3 py-1.5 font-medium">
                      {columnLabels[key]}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {actions.map((row) => (
                  <tr key={row.id} className="border-b border-border/50 last:border-0 hover:bg-muted/40">
                    {columns.map((key) => {
                      const value =
                        key === "id"
                          ? row.id
                          : key === "name"
                            ? row.name
                            : key === "workspace"
                              ? row.workspace
                              : key === "listen"
                                ? row.listen
                                : row.action
                      return (
                        <td
                          key={key}
                          className="max-w-[220px] truncate whitespace-nowrap px-3 py-1.5"
                          title={String(value)}
                        >
                          {value}
                        </td>
                      )
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>

        <Card className="flex min-h-0 flex-[2] flex-col gap-0 py-0">
          <CardHeader className="flex flex-row items-center justify-between gap-2 px-3 py-2">
            <CardTitle className="text-sm font-medium">{text("topkTitle", "Top-K frequency")}</CardTitle>
            <Select value={String(topk)} onValueChange={(value) => setTopk(Number(value))}>
              <SelectTrigger size="sm" className="h-7 w-20" aria-label={text("topkAria", "Top-K frequency")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" align="end">
                {kChoices.map((k) => (
                  <SelectItem key={k} value={String(k)}>
                    {k}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </CardHeader>
          <CardContent className="min-h-0 flex-1 px-2 pb-2">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={topkSlice} margin={{ top: 4, right: 8, bottom: 0, left: -20 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
                <XAxis dataKey="name" tick={{ fontSize: 10 }} interval={0} angle={-25} textAnchor="end" height={40} />
                <YAxis tick={{ fontSize: 10 }} allowDecimals={false} />
                <Tooltip contentStyle={{ fontSize: 12, borderRadius: 8 }} />
                <Bar dataKey="count" fill="var(--primary)" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

/**
 * footer 左侧: proxy 总数(图标 badge)。
 *
 * 与视图数据同源(proxyStatusCounts 求和), 不额外引入状态;
 * 文案走 static(contentArea.home.analysis.proxy.total*)。
 */
export function ProxyFooterInfo() {
  const { Value } = useStatic()
  const label = (path: string, fallback: string): string => {
    const value = Value(path)
    return typeof value === "string" ? value : fallback
  }
  const [total, setTotal] = React.useState(0)

  React.useEffect(() => {
    let alive = true
    void (async () => {
      try {
        const value = await loadProxyTotal()
        if (alive) setTotal(value)
      } catch {
        // 保持 0
      }
    })()
    return () => {
      alive = false
    }
  }, [])

  return (
    <Badge
      variant="secondary"
      className="gap-1 text-xs font-normal"
      aria-label={`${label("contentArea.home.analysis.proxy.totalAria", "Total proxies")}: ${total}`}
    >
      <NetworkIcon className="size-3.5" />
      <span>{label("contentArea.home.analysis.proxy.totalLabel", "Proxies")}</span>
      <span className="font-medium tabular-nums text-foreground">{total}</span>
    </Badge>
  )
}
