import * as React from "react"
import { cn } from "cn"

import { useStatic } from "@/components/custom/share/provider/static-provider"
import { loadSystemInfo, type SystemRow } from "@/lib/analysis"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

/**
 * System 分析视图(纯 UI, mock 数据; 见 docs/design/v23.md §72)。
 *
 * 布局: 整体一个大 Card, 内部一张 [id, key, value] 的平台信息表。
 * 约定: 滚动条隐藏、表头 sticky、单行截断 + title(与其它 view 一致)。
 */
export default function SystemAnalysisView() {
  const { Value } = useStatic()
  const [rows, setRows] = React.useState<SystemRow[]>([])

  React.useEffect(() => {
    let alive = true
    void (async () => {
      try {
        const items = await loadSystemInfo()
        if (alive && items.length > 0) setRows(items)
      } catch {
        // 保持 mock
      }
    })()
    return () => {
      alive = false
    }
  }, [])
  const text = (path: string, fallback: string): string => {
    const value = Value(path)
    return typeof value === "string" ? value : fallback
  }

  return (
    <div className="flex min-h-0 flex-1 p-2">
      <Card className="flex min-h-0 w-full min-w-0 flex-col gap-0 py-0">
        <CardHeader className="px-3 py-2">
          <CardTitle className="text-sm font-medium">
            {text("contentArea.home.analysis.system.title", "System information")}
          </CardTitle>
        </CardHeader>
        <CardContent
          className={cn(
            "min-h-0 flex-1 overflow-auto px-0 pb-0",
            // 滚动条一律隐藏(不占布局)
            "[&::-webkit-scrollbar]:h-0 [&::-webkit-scrollbar]:w-0",
            "[&::-webkit-scrollbar-thumb]:bg-transparent",
            "[&::-webkit-scrollbar-track]:bg-transparent",
          )}
        >
          <table className="w-full min-w-full caption-bottom text-xs">
            <thead className="sticky top-0 z-10 bg-background">
              <tr className="border-b text-left text-muted-foreground">
                <th className="w-16 whitespace-nowrap px-3 py-1.5 font-medium">
                  {text("contentArea.home.analysis.system.columns.id", "ID")}
                </th>
                <th className="w-56 whitespace-nowrap px-3 py-1.5 font-medium">
                  {text("contentArea.home.analysis.system.columns.key", "Key")}
                </th>
                <th className="whitespace-nowrap px-3 py-1.5 font-medium">
                  {text("contentArea.home.analysis.system.columns.value", "Value")}
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.id} className="border-b border-border/50 last:border-0 hover:bg-muted/40">
                  <td className="max-w-[80px] truncate whitespace-nowrap px-3 py-1.5 tabular-nums" title={String(row.id)}>
                    {row.id}
                  </td>
                  <td className="max-w-[280px] truncate whitespace-nowrap px-3 py-1.5" title={row.key}>
                    {row.key}
                  </td>
                  <td className="max-w-[520px] truncate whitespace-nowrap px-3 py-1.5 text-muted-foreground" title={row.value}>
                    {row.value}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </CardContent>
      </Card>
    </div>
  )
}
