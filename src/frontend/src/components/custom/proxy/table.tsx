import { CheckIcon, MoreHorizontalIcon } from "lucide-react"
import { cn } from "cn"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  proxyApplications,
  proxyStatusActions,
  type ProxyAction,
  type ProxyApplication,
} from "@/mock/proxies"
import { useStatic, staticValue } from "@/components/custom/share/provider/static-provider"
import { toStatus, type Proxy, type ProxyStatus } from "@/components/custom/proxy/model"
import { formatDateTime } from "@/lib/format"

/** 状态 -> 字体色 (底色统一 bg-secondary, 随主题明暗变化; 颜色来自 index.css 的 --status-* token)。 */
const statusText: Record<ProxyStatus, string> = {
  running: "text-status-running",
  stopped: "text-status-stopped",
  error: "text-status-error",
  exception: "text-status-exception",
}

/**
 * 层级约定 (bg 必须不透明, 否则滚动内容会透出):
 *   表头普通列 z-20 < 表头冻结列 z-30, 表格体冻结列 z-10 < 表头任何列。
 * 注意: 拆成独立常量 —— 若把 z 写在同一元素的多个 class 串里传给 cn(), twMerge 会按"后者胜"
 * 合并 z 值 (实测 cn("…z-20…", "…z-10…") => z-10), 表头与表格体层级相同后会被表格体盖住。
 */
const headClass = "sticky top-0 z-20 bg-background"
const headFrozenClass = "sticky top-0 right-0 z-30 bg-background"
const frozenClass = "sticky right-0 z-10 bg-background"

/**
 * 代理数据表: 多选 / 名称 / 目标 / 状态 / 更新时间 + 冻结的操作列。
 * 容器尺寸固定, 行数超出时表体滚动; 列超出时其余列横向滚动, 表头与操作列保持可见。
 * 行操作菜单: [Edit / Inspect / Delete] + 二级 [Status] / [Application]。
 */
export default function ProxyTable({
  rows,
  selectedIds,
  onToggleRow,
  onToggleAll,
  onEdit,
  onInspect,
  onDelete,
  onStatus,
  onApplication,
}: {
  rows: Proxy[]
  selectedIds: Set<string>
  onToggleRow: (id: string) => void
  onToggleAll: () => void
  onEdit: (row: Proxy) => void
  onInspect: (row: Proxy) => void
  onDelete: (row: Proxy) => void
  onStatus: (row: Proxy, action: ProxyAction) => void
  onApplication: (row: Proxy, application: ProxyApplication) => void
}) {
  const { Value } = useStatic()
  const allSelected = rows.length > 0 && rows.every((row) => selectedIds.has(row.id))

  return (
    <div className="min-h-0 flex-1 overflow-auto [&_[data-slot=table-container]]:overflow-visible">
      <Table>
        <TableHeader className="bg-background">
          <TableRow>
            <TableHead className={cn(headClass, "w-10")}>
              {/* 多选只用"是否显示 Check"表达, 不做色调/底色差异; 常显边框避免控件隐身 */}
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-6 border"
                aria-pressed={allSelected}
                aria-label={staticValue("contentArea.proxy.selectAll")}
                onClick={onToggleAll}
              >
                {allSelected && <CheckIcon className="size-3.5" />}
              </Button>
            </TableHead>
            <TableHead className={headClass}>{staticValue("contentArea.proxy.columns.name")}</TableHead>
            <TableHead className={headClass}>{staticValue("contentArea.proxy.columns.target")}</TableHead>
            <TableHead className={cn(headClass, "w-28")}>{staticValue("contentArea.proxy.columns.status")}</TableHead>
            <TableHead className={cn(headClass, "w-44")}>{staticValue("contentArea.proxy.columns.updated")}</TableHead>
            <TableHead className={cn(headFrozenClass, "w-12")} />
          </TableRow>
        </TableHeader>
        <TableBody>
          {/* 不给行设 data-state="selected": ui/table 的 TableRow 会据此加 bg-muted, 与本页"无色调差异"的约定冲突 */}
          {rows.map((row) => (
            <TableRow key={row.id}>
              <TableCell>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-6 border"
                  aria-pressed={selectedIds.has(row.id)}
                  aria-label={`${staticValue("contentArea.proxy.rowSelect") ?? ""}${row.name}`}
                  onClick={() => onToggleRow(row.id)}
                >
                  {selectedIds.has(row.id) && <CheckIcon className="size-3.5" />}
                </Button>
              </TableCell>
              <TableCell className="font-medium">{row.name}</TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">
                {row.target}
              </TableCell>
              <TableCell>
                <Badge variant="secondary" className={statusText[toStatus(row.status)]}>
                  {Value(`contentArea.proxy.statusLabel.${row.status}`) as string}
                </Badge>
              </TableCell>
              <TableCell className="text-muted-foreground tabular-nums">
                {formatDateTime(row.updatedAt)}
              </TableCell>
              <TableCell className={frozenClass}>
                <DropdownMenu>
                  {/*
                    触发链路: Button(asChild) -> DropdownMenuTrigger。
                    ui/button 是 React 19 风格的普通函数组件 (无 forwardRef), 若反过来写成
                    <DropdownMenuTrigger asChild><Button>, React 18 下 ref 传不进 Radix,
                    锚点为空 -> 菜单虽 open 却被定位到屏幕外 (表现为"点了没反应")。
                  */}
                  <Button
                    asChild
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    aria-label={`${staticValue("contentArea.proxy.actions") ?? ""}${row.name}`}
                  >
                    <DropdownMenuTrigger>
                      <MoreHorizontalIcon />
                    </DropdownMenuTrigger>
                  </Button>
                  <DropdownMenuContent align="end" className="w-44">
                    <DropdownMenuItem onSelect={() => onEdit(row)}>{staticValue("contentArea.proxy.action.edit")}</DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => onInspect(row)}>
                      {staticValue("contentArea.proxy.action.inspect")}
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => onDelete(row)}>{staticValue("contentArea.proxy.action.delete")}</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuSub>
                      <DropdownMenuSubTrigger>{staticValue("contentArea.proxy.action.status")}</DropdownMenuSubTrigger>
                      <DropdownMenuSubContent>
                        {proxyStatusActions.map((action) => (
                          <DropdownMenuItem
                            key={action.value}
                            onSelect={() => onStatus(row, action.value)}
                          >
                            {staticValue(`contentArea.proxy.actionLabel.${action.value}`) as string}
                          </DropdownMenuItem>
                        ))}
                      </DropdownMenuSubContent>
                    </DropdownMenuSub>
                    <DropdownMenuSub>
                      <DropdownMenuSubTrigger>{staticValue("contentArea.proxy.action.application")}</DropdownMenuSubTrigger>
                      <DropdownMenuSubContent>
                        {proxyApplications.map((app) => (
                          <DropdownMenuItem
                            key={app.value}
                            onSelect={() => onApplication(row, app.value)}
                          >
                            <app.icon />
                            <span>{app.label}</span>
                          </DropdownMenuItem>
                        ))}
                      </DropdownMenuSubContent>
                    </DropdownMenuSub>
                  </DropdownMenuContent>
                </DropdownMenu>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
