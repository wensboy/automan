import * as React from "react"
import { toast } from "sonner"

import ProxyFooter from "@/components/custom/proxy/footer"
import ProxySheet from "@/components/custom/proxy/proxy-sheet"
import ProxyTable from "@/components/custom/proxy/table"
import ProxyToolbar from "@/components/custom/proxy/toolbar"
import {
  proxies,
  proxyPageSizes,
  proxyStatusActions,
  type ProxyAction,
  type ProxyApplication,
  type ProxyRow,
} from "@/mock/proxies"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/**
 * 代理 (nav-Main: Proxy): header (搜索 + 工具按钮) + body (数据表与 footer 黏合)。
 * 页面持有数据与列表状态 (行数据 / 搜索 / 分页 / 选中 / 编辑目标), 组件只负责渲染。
 * 行操作: edit 打开编辑 sheet; inspect 暂为 toast; delete 真删; status/application 直接改该行状态。
 */
export default function Proxy() {
  const [rows, setRows] = React.useState<ProxyRow[]>(proxies)
  const [query, setQuery] = React.useState("")
  const [page, setPage] = React.useState(1)
  const [rowsPerPage, setRowsPerPage] = React.useState(proxyPageSizes[0])
  const [sheetOpen, setSheetOpen] = React.useState(false)
  const [editRow, setEditRow] = React.useState<ProxyRow | null>(null)
  const [selectedIds, setSelectedIds] = React.useState<Set<string>>(new Set())

  const filtered = React.useMemo(() => {
    const keyword = query.trim().toLowerCase()
    if (keyword === "") return rows
    return rows.filter((row) =>
      [row.name, row.target].some((field) => field.toLowerCase().includes(keyword)),
    )
  }, [rows, query])

  const pageCount = Math.max(1, Math.ceil(filtered.length / rowsPerPage))
  const current = Math.min(page, pageCount)
  const visibleRows = filtered.slice((current - 1) * rowsPerPage, current * rowsPerPage)
  const rangeStart = filtered.length === 0 ? 0 : (current - 1) * rowsPerPage + 1
  const rangeEnd = Math.min(current * rowsPerPage, filtered.length)

  // 页脚统计: 与列表同源 (当前筛选结果)
  const counts = React.useMemo(
    () => ({
      running: filtered.filter((row) => row.status === "running").length,
      exception: filtered.filter((row) => row.status === "exception").length,
      error: filtered.filter((row) => row.status === "error").length,
    }),
    [filtered],
  )

  const toggleRow = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  // 全选/全不选只针对当前页的行 (选中集合本身跨页保留)
  const toggleAll = () => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      const allSelected =
        visibleRows.length > 0 && visibleRows.every((row) => prev.has(row.id))
      for (const row of visibleRows) {
        if (allSelected) {
          next.delete(row.id)
        } else {
          next.add(row.id)
        }
      }
      return next
    })
  }

  const openEdit = (row: ProxyRow) => {
    setEditRow(row)
    setSheetOpen(true)
  }

  const removeRow = (row: ProxyRow) => {
    setRows((prev) => prev.filter((item) => item.id !== row.id))
    setSelectedIds((prev) => {
      const next = new Set(prev)
      next.delete(row.id)
      return next
    })
    toast.success(`${staticValue("contentArea.proxy.deleted") ?? ""}${row.name}`)
  }

  const applyStatus = (row: ProxyRow, action: ProxyAction) => {
    const target = proxyStatusActions.find((item) => item.value === action)
    if (!target) return
    setRows((prev) =>
      prev.map((item) => (item.id === row.id ? { ...item, status: target.status } : item)),
    )
    toast.success(`${staticValue(`contentArea.proxy.actionLabel.${target.value}`) as string} ${row.name}`)
  }

  const applyApplication = (row: ProxyRow, application: ProxyApplication) => {
    setRows((prev) =>
      prev.map((item) => (item.id === row.id ? { ...item, application } : item)),
    )
    toast.success(`${staticValue(`contentArea.proxy.applicationLabel.${application}`) as string} for ${row.name}`)
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 p-4 pt-0">
      <ProxyToolbar
        query={query}
        onQueryChange={(value) => {
          setQuery(value)
          setPage(1)
        }}
        onAdd={() => {
          setEditRow(null)
          setSheetOpen(true)
        }}
        onRefresh={() => toast.success(staticValue("contentArea.proxy.refreshed") ?? "")}
        onClear={() => {
          setQuery("")
          setPage(1)
        }}
        onStart={() => toast.success(staticValue("contentArea.proxy.started") ?? "")}
        onStop={() => toast.success(staticValue("contentArea.proxy.stopped") ?? "")}
      />
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border">
        <ProxyTable
          rows={visibleRows}
          selectedIds={selectedIds}
          onToggleRow={toggleRow}
          onToggleAll={toggleAll}
          onEdit={openEdit}
          onInspect={(row) => toast(`${staticValue("contentArea.proxy.inspected") ?? ""}${row.name}`)}
          onDelete={removeRow}
          onStatus={applyStatus}
          onApplication={applyApplication}
        />
        <ProxyFooter
          rangeStart={rangeStart}
          rangeEnd={rangeEnd}
          total={filtered.length}
          counts={counts}
          page={current}
          pageCount={pageCount}
          pageSizes={proxyPageSizes}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(size) => {
            setRowsPerPage(size)
            setPage(1)
          }}
          onPageChange={setPage}
        />
      </div>
      {/* key 随编辑目标变化 -> 切换目标时重置 sheet 内部状态 (而不是复用上一行的草稿) */}
      <ProxySheet
        key={editRow?.id ?? "add"}
        open={sheetOpen}
        onOpenChange={(open) => {
          setSheetOpen(open)
          if (!open) setEditRow(null)
        }}
        row={editRow ?? undefined}
      />
    </div>
  )
}
