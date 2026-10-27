import * as React from "react"

import LogAnalysisView, { LogFooterInfo } from "@/components/custom/home/views/log"
import ProxyAnalysisView, { ProxyFooterInfo } from "@/components/custom/home/views/proxy"
import ServiceAnalysisView, { ServiceFooterInfo } from "@/components/custom/home/views/service"
import SystemAnalysisView from "@/components/custom/home/views/system"
import EmptyState from "@/components/custom/share/empty-state"
import { useStatic } from "@/components/custom/share/provider/static-provider"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

/**
 * 分析视图(home > Analysis): 多视图切换, 用于监控。
 *
 * 布局(见 docs/design/v23.md §60):
 *   v_u->d( viewArea(flex-1), footer(h_l->r() 左占位, h_r->l(select)) )
 * 约定:
 *   - 右侧 select 决定 viewArea 显示哪个视图, 同时决定 footer 左侧内容(当前占位);
 *   - 各视图内容后续逐个补齐, 现阶段统一 EmptyState;
 *   - 图表库(recharts)在真正落图时再引入, 骨架阶段不增加包体。
 */
const viewKeys = ["proxy", "record", "automation", "log", "service", "system"] as const

type ViewKey = (typeof viewKeys)[number]

export default function AnalysisView() {
  const { Value } = useStatic()
  const [view, setView] = React.useState<ViewKey>("proxy")

  const labels = React.useMemo(() => {
    const items = Value("contentArea.home.analysis.views") ?? []
    const map: Record<string, string> = {}
    for (const item of items) map[item.value] = item.label
    return map
  }, [Value])

  const empty = Value("contentArea.home.analysis.empty") ?? ""

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* viewArea: 后续按 view 渲染各自的监控面板 */}
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        {view === "proxy" ? (
          <ProxyAnalysisView />
        ) : view === "log" ? (
          <LogAnalysisView />
        ) : view === "system" ? (
          <SystemAnalysisView />
        ) : view === "service" ? (
          <ServiceAnalysisView />
        ) : (
          <EmptyState title={labels[view] ?? view} description={empty} />
        )}
      </div>

      {/* footer: 左 = 当前视图相关操作(占位), 右 = 视图切换
          右侧 pr-2 与 viewArea 的卡片内边距(p-2)一致 => select 右缘与卡片右缘对齐 */}
      <div className="flex h-9 shrink-0 items-center justify-between gap-2 border-t pr-2 pl-2">
          <div className="min-w-0 truncate text-xs text-muted-foreground">
            {view === "proxy" ? (
              <ProxyFooterInfo />
            ) : view === "service" ? (
              <ServiceFooterInfo />
            ) : view === "log" ? (
              <LogFooterInfo />
            ) : (
              (labels[view] ?? view)
            )}
          </div>
          <Select value={view} onValueChange={(value) => setView(value as ViewKey)}>
            <SelectTrigger size="sm" className="h-7 w-36" aria-label={Value("contentArea.home.analysis.switchAria") ?? ""}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent position="popper" align="end">
              {viewKeys.map((key) => (
                <SelectItem key={key} value={key}>
                  {labels[key] ?? key}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
      </div>
    </div>
  )
}
