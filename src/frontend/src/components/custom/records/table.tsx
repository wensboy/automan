import { CheckIcon, MoreHorizontalIcon } from "lucide-react"
import { cn } from "cn"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
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
import { type RecordRow, type RecordStatus } from "@/mock/records"
import { useStatic, staticValue } from "@/components/custom/share/provider/static-provider"

/** 状态 -> 字体色 (底色统一 bg-secondary, 随主题明暗变化; 颜色来自 index.css 的 --status-* token)。 */
const statusText: Record<RecordStatus, string> = {
  running: "text-status-running",
  stopped: "text-status-stopped",
  error: "text-status-error",
  exception: "text-status-exception",
}

/**
 * 层级约定 (bg 必须不透明, 否则滚动内容会透出):
 *   表头普通列 z-20 < 表头冻结列 z-30, 表格体冻结列 z-10 < 表头任何列。
 * 注意: 拆成独立常量 —— 若把 z 写在同一元素的多个 class 串里传给 cn(), twMerge 会按"后者胜"合并。
 */
const headClass = "sticky top-0 z-20 bg-background"
const headFrozenClass = "sticky top-0 right-0 z-30 bg-background"
const frozenClass = "sticky right-0 z-10 bg-background"

/**
 * 记录数据表: 多选 / 名称 / 目标 / 状态 / 更新时间 + 冻结的操作列。
 * 行操作只有一级 [Edit / Inspect / Delete] (记录页没有 status/application 两个二级菜单)。
 * 与代理页同款视觉, 但组件独立实现, 两端互不影响。
 */
export default function RecordsTable({
  rows,
  selectedIds,
  onToggleRow,
  onToggleAll,
  onEdit,
  onInspect,
  onDelete,
}: {
  rows: RecordRow[]
  selectedIds: Set<string>
  onToggleRow: (id: string) => void
  onToggleAll: () => void
  onEdit: (row: RecordRow) => void
  onInspect: (row: RecordRow) => void
  onDelete: (row: RecordRow) => void
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
                aria-label={staticValue("contentArea.records.selectAll")}
                onClick={onToggleAll}
              >
                {allSelected && <CheckIcon className="size-3.5" />}
              </Button>
            </TableHead>
            <TableHead className={headClass}>{staticValue("contentArea.records.columns.name")}</TableHead>
            <TableHead className={headClass}>{staticValue("contentArea.records.columns.target")}</TableHead>
            <TableHead className={cn(headClass, "w-28")}>{staticValue("contentArea.records.columns.status")}</TableHead>
            <TableHead className={cn(headClass, "w-44")}>{staticValue("contentArea.records.columns.updated")}</TableHead>
            <TableHead className={cn(headFrozenClass, "w-12")} />
          </TableRow>
        </TableHeader>
        <TableBody>
          {/* 不给行设 data-state="selected": ui/table 的 TableRow 会据此加 bg-muted, 与"无色调差异"的约定冲突 */}
          {rows.map((row) => (
            <TableRow key={row.id}>
              <TableCell>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-6 border"
                  aria-pressed={selectedIds.has(row.id)}
                  aria-label={`${staticValue("contentArea.records.rowSelect") ?? ""}${row.name}`}
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
                <Badge variant="secondary" className={statusText[row.status]}>
                  {Value(`contentArea.records.statusLabel.${row.status}`)}
                </Badge>
              </TableCell>
              <TableCell className="text-muted-foreground">{row.updatedAt}</TableCell>
              <TableCell className={frozenClass}>
                <DropdownMenu>
                  {/*
                    触发链路必须是 Button(asChild) -> DropdownMenuTrigger:
                    ui/button 是 React 19 风格普通函数组件(无 forwardRef), 反向嵌套会丢 ref,
                    Radix 锚点为空 -> 菜单被定位到屏幕外 (表现为"点了没反应")。
                  */}
                  <Button
                    asChild
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    aria-label={`${staticValue("contentArea.records.actions") ?? ""}${row.name}`}
                  >
                    <DropdownMenuTrigger>
                      <MoreHorizontalIcon />
                    </DropdownMenuTrigger>
                  </Button>
                  <DropdownMenuContent align="end" className="w-40">
                    <DropdownMenuItem onSelect={() => onEdit(row)}>{staticValue("contentArea.records.action.edit")}</DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => onInspect(row)}>
                      {staticValue("contentArea.records.action.inspect")}
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => onDelete(row)}>{staticValue("contentArea.records.action.delete")}</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </TableCell>
            </TableRow>
          ))}
          {rows.length === 0 && (
            <TableRow>
              <TableCell
                colSpan={6}
                className="py-10 text-center text-sm text-muted-foreground"
              >
                {staticValue("contentArea.records.empty")}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}
