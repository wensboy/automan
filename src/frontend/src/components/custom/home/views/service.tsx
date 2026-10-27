import * as React from "react"
import { cn } from "cn"
import { ActivityIcon } from "lucide-react"
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"

import EmptyState from "@/components/custom/share/empty-state"
import { useStatic } from "@/components/custom/share/provider/static-provider"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { loadServiceSnapshot } from "@/lib/analysis"
import {
  formatMetric,
  serviceIntervalMs,
  servicePid,
  type Sample,
  type SeriesDef,
  type ServiceResource,
} from "@/mock/analysis-service"

/**
 * Service 分析视图(见 docs/design/v25.md §7/§9)。
 *
 * 布局: h_l->r( 垂直 Tabs 1, Card 2 ); Card 内 chart 2 : table 1。
 * 数据: 后端 ServiceSnapshot(10s 轮询); 失败时回落 mock, 保证视图可用。
 */

const seriesColors = ["var(--primary)", "var(--status-running)", "var(--status-stopped)", "var(--status-error)"]

const scrollbarClass = cn(
  "min-h-0 overflow-auto",
  "[&::-webkit-scrollbar]:h-0 [&::-webkit-scrollbar]:w-0",
  "[&::-webkit-scrollbar-thumb]:bg-transparent",
  "[&::-webkit-scrollbar-track]:bg-transparent",
)

const yDomain: [((min: number) => number), ((max: number) => number)] = [
  (min) => Math.max(0, Math.floor(min - Math.max(Math.abs(min) * 0.1, 1))),
  (max) => Math.ceil(max + Math.max(Math.abs(max) * 0.1, 1)),
]

const xTickProps = { interval: "preserveStartEnd" as const, minTickGap: 16, fontSize: 10 }

function chartTypeOf(resource: ServiceResource): "line" | "bar" | "area" {
  if (resource === "gc") return "bar"
  if (resource === "heap") return "area"
  return "line"
}

type ChartRow = Record<string, number>

export default function ServiceAnalysisView() {
  const { Value } = useStatic()
  const [resource, setResource] = React.useState<ServiceResource>("cpu")
  const [bridgeService, setBridgeService] = React.useState("node")

  const label = (path: string, fallback: string): string => {
    const value = Value(path)
    return typeof value === "string" ? value : fallback
  }

  const resources = React.useMemo(() => {
    const items = Value("contentArea.home.analysis.service.resources")
    return Array.isArray(items) ? items : []
  }, [Value])

  const [series, setSeries] = React.useState<SeriesDef[]>([])
  const [samples, setSamples] = React.useState<Sample[]>([])
  const [bridgeServices, setBridgeServices] = React.useState<{ value: string; label: string }[]>([])

  // 后端快照: 10s 轮询(与后端采样间隔一致); 失败保持上一次/mock
  React.useEffect(() => {
    let alive = true
    const pull = async () => {
      try {
        const snapshot = await loadServiceSnapshot(bridgeService)
        if (!alive || !snapshot) return
        const metric = resource === "bridge" ? snapshot.bridge : snapshot.resources?.[resource]
        if (metric) {
          const nextSeries = (metric.series ?? []) as SeriesDef[]
          const nextSamples = (metric.samples ?? []) as Sample[]
          if (nextSeries.length > 0) setSeries(nextSeries)
          if (nextSamples.length > 0) setSamples(nextSamples)
        }
        const services = (snapshot.bridge?.services ?? []).map((item) => ({
          value: String(item.value),
          label: String(item.label),
        }))
        if (services.length > 0) setBridgeServices(services)
      } catch {
        // 回落 mock
      }
    }
    void pull()
    const timer = window.setInterval(() => void pull(), serviceIntervalMs)
    return () => {
      alive = false
      window.clearInterval(timer)
    }
  }, [resource, bridgeService])

  // 切资源时清空, 等后端快照返回(避免残留上一个资源的数据)
  React.useEffect(() => {
    setSeries([])
    setSamples([])
  }, [resource])

  const current = resources.find((item) => String(item.value) === resource)
  const title = current ? String(current.label) : resource.toUpperCase()

  // bridge 只看 rate; io 图表只看 disk busy(其它指标进表格)
  const chartSeries =
    resource === "bridge"
      ? series.filter((item) => item.key === "rate")
      : resource === "io"
        ? series.filter((item) => item.key === "readTotal" || item.key === "writeTotal")
        : series

  const chartData = React.useMemo<ChartRow[]>(
    () => samples.map((sample, index) => ({ interval: index + 1, ...sample.values })),
    [samples],
  )

  // 表格: 每个 metric 唯一一行, value 取最新采样(不再按 interval 展开)
  const tableRows = React.useMemo(() => {
    if (samples.length === 0) return []
    const latest = samples[samples.length - 1]
    return series.map((def, index) => ({
      id: index + 1,
      metrics: def.label,
      value: formatMetric(Number(latest.values[def.key] ?? 0), def),
    }))
    }, [samples, series])

  const columns = {
    id: label("contentArea.home.analysis.service.columns.id", "ID"),
    metrics: label("contentArea.home.analysis.service.columns.metrics", "Metrics"),
    value: label("contentArea.home.analysis.service.columns.value", "Value"),
  }

  const chartType = chartTypeOf(resource)

  const chart = (
    <ResponsiveContainer width="100%" height="100%">
      {chartType === "bar" ? (
        <BarChart data={chartData} margin={{ top: 4, right: 8, bottom: 0, left: -18 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
          <XAxis dataKey="interval" tick={xTickProps} />
          <YAxis tick={{ fontSize: 10 }} domain={yDomain} allowDataOverflow={false} />
          <Tooltip contentStyle={{ fontSize: 12, borderRadius: 8 }} labelFormatter={(v) => `#${v}`} />
          {chartSeries.map((def, index) => (
            <Bar key={def.key} dataKey={def.key} name={def.label} fill={seriesColors[index]} radius={[3, 3, 0, 0]} />
          ))}
        </BarChart>
      ) : chartType === "area" ? (
        <AreaChart data={chartData} margin={{ top: 4, right: 8, bottom: 0, left: -18 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
          <XAxis dataKey="interval" tick={xTickProps} />
          <YAxis tick={{ fontSize: 10 }} domain={yDomain} allowDataOverflow={false} />
          <Tooltip contentStyle={{ fontSize: 12, borderRadius: 8 }} labelFormatter={(v) => `#${v}`} />
          {chartSeries.map((def, index) => (
            <Area
              key={def.key}
              dataKey={def.key}
              name={def.label}
              stroke={seriesColors[index]}
              fill={seriesColors[index]}
              fillOpacity={0.15}
              strokeWidth={1.5}
            />
          ))}
        </AreaChart>
      ) : (
        <LineChart data={chartData} margin={{ top: 4, right: 8, bottom: 0, left: -18 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
          <XAxis dataKey="interval" tick={xTickProps} />
          <YAxis tick={{ fontSize: 10 }} domain={yDomain} allowDataOverflow={false} />
          <Tooltip contentStyle={{ fontSize: 12, borderRadius: 8 }} labelFormatter={(v) => `#${v}`} />
          {chartSeries.map((def, index) => (
            <Line
              key={def.key}
              type="monotone"
              dataKey={def.key}
              name={def.label}
              stroke={seriesColors[index]}
              strokeWidth={1.5}
              dot={false}
              isAnimationActive={false}
            />
          ))}
        </LineChart>
      )}
    </ResponsiveContainer>
  )

  const bridgeOptions = bridgeServices.length > 0
    ? bridgeServices
    : ((Value("contentArea.home.analysis.service.bridge.services") ?? []) as { value: string; label: string }[]).map(
        (item) => ({ value: String(item.value), label: String(item.label) }),
      )

  return (
    <Tabs
      value={resource}
      onValueChange={(value) => setResource(value as ServiceResource)}
      orientation="vertical"
      className="flex min-h-0 flex-1 gap-2 p-2"
    >
      <TabsList className="min-h-0 min-w-0 flex-[1] items-stretch justify-start gap-1 p-1">
        {resources.map((item) => (
          <TabsTrigger key={String(item.value)} value={String(item.value)} className="h-8 flex-none justify-start px-2">
            {String(item.label)}
          </TabsTrigger>
        ))}
      </TabsList>

      <Card className="flex min-h-0 min-w-0 flex-[2] flex-col gap-0 py-0">
        <CardHeader className="flex flex-row items-center justify-between gap-2 px-3 py-2">
          <CardTitle className="text-sm font-medium">{title}</CardTitle>
          {resource === "bridge" && (
            <Select value={bridgeService} onValueChange={setBridgeService}>
              <SelectTrigger
                size="sm"
                className="h-7 w-32"
                aria-label={label("contentArea.home.analysis.service.bridge.selectAria", "Bridge service")}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" align="end">
                {bridgeOptions.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </CardHeader>
        <CardContent className="flex min-h-0 flex-1 flex-col gap-2 px-3 pb-3">
          {series.length === 0 ? (
            <EmptyState
              title={title}
              description={label("contentArea.home.analysis.service.empty", "Metrics for this resource are on the way.")}
            />
          ) : (
            <>
              <div className="min-h-0 flex-[2]">{chart}</div>
              <div className={cn(scrollbarClass, "min-h-0 flex-[1] rounded-md border")}>
                <table className="w-full min-w-full caption-bottom text-xs">
                  <thead className="sticky top-0 z-10 bg-background">
                    <tr className="border-b text-left text-muted-foreground">
                      <th className="w-16 whitespace-nowrap px-3 py-1.5 font-medium">{columns.id}</th>
                      <th className="whitespace-nowrap px-3 py-1.5 font-medium">{columns.metrics}</th>
                      <th className="whitespace-nowrap px-3 py-1.5 font-medium">{columns.value}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {tableRows.map((row) => (
                      <tr key={row.id} className="border-b border-border/50 last:border-0 hover:bg-muted/40">
                        <td className="whitespace-nowrap px-3 py-1.5 tabular-nums">{row.id}</td>
                        <td className="max-w-[220px] truncate whitespace-nowrap px-3 py-1.5" title={row.metrics}>
                          {row.metrics}
                        </td>
                        <td
                          className="max-w-[160px] truncate whitespace-nowrap px-3 py-1.5 tabular-nums text-muted-foreground"
                          title={row.value}
                        >
                          {row.value}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </Tabs>
  )
}

/** footer 左侧: 进程号(图标 badge)。 */
export function ServiceFooterInfo() {
  const { Value } = useStatic()
  const [pid, setPid] = React.useState(servicePid)

  React.useEffect(() => {
    let alive = true
    void (async () => {
      try {
        const snapshot = await loadServiceSnapshot("node")
        if (alive && snapshot?.pid) setPid(Number(snapshot.pid))
      } catch {
        // 保持 0
      }
    })()
    return () => {
      alive = false
    }
  }, [])
  const label = (path: string, fallback: string): string => {
    const value = Value(path)
    return typeof value === "string" ? value : fallback
  }
  return (
    <Badge
      variant="secondary"
      className="gap-1 text-xs font-normal"
      aria-label={`${label("contentArea.home.analysis.service.pidAria", "Process id")}: ${pid}`}
    >
      <ActivityIcon className="size-3.5" />
      <span>{label("contentArea.home.analysis.service.pidLabel", "PID")}</span>
      <span className="font-medium tabular-nums text-foreground">{pid}</span>
    </Badge>
  )
}
