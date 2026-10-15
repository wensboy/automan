import * as React from "react"
import { toast } from "sonner"

import RecordsFooter from "@/components/custom/records/footer"
import RecordSheet from "@/components/custom/records/record-sheet"
import RecordsTable from "@/components/custom/records/table"
import RecordsToolbar from "@/components/custom/records/toolbar"
import { records, recordPageSizes, type RecordRow } from "@/mock/records"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/**
 * 记录 (nav-Main: Records): header (搜索 + 刷新/清除) + body (数据表与 footer 黏合)。
 * 与代理页同款视觉与交互, 但组件与数据均独立实现 (两端可各自演进, 互不牵制)。
 * 页面持有数据与列表状态; inspect 暂为 toast, 删除真删, edit 打开编辑 sheet。
 */
export default function Records() {
  const [rows, setRows] = React.useState<RecordRow[]>(records)
  const [query, setQuery] = React.useState("")
  const [page, setPage] = React.useState(1)
  const [rowsPerPage, setRowsPerPage] = React.useState(recordPageSizes[0])
  const [editRow, setEditRow] = React.useState<RecordRow | null>(null)
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

  const removeRow = (row: RecordRow) => {
    setRows((prev) => prev.filter((item) => item.id !== row.id))
    setSelectedIds((prev) => {
      const next = new Set(prev)
      next.delete(row.id)
      return next
    })
    toast.success(`${staticValue("contentArea.records.deleted") ?? ""}${row.name}`)
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 p-4 pt-0">
      <RecordsToolbar
        query={query}
        onQueryChange={(value) => {
          setQuery(value)
          setPage(1)
        }}
        onRefresh={() => toast.success(staticValue("contentArea.records.refreshed") ?? "")}
        onClear={() => {
          setQuery("")
          setPage(1)
        }}
      />
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border">
        <RecordsTable
          rows={visibleRows}
          selectedIds={selectedIds}
          onToggleRow={toggleRow}
          onToggleAll={toggleAll}
          onEdit={(row) => setEditRow(row)}
          onInspect={(row) => toast(`${staticValue("contentArea.records.inspected") ?? ""}${row.name}`)}
          onDelete={removeRow}
        />
        <RecordsFooter
          rangeStart={rangeStart}
          rangeEnd={rangeEnd}
          total={filtered.length}
          page={current}
          pageCount={pageCount}
          pageSizes={recordPageSizes}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(size) => {
            setRowsPerPage(size)
            setPage(1)
          }}
          onPageChange={setPage}
        />
      </div>
      {editRow && (
        <RecordSheet
          key={editRow.id}
          open
          onOpenChange={(open) => {
            if (!open) setEditRow(null)
          }}
          row={editRow}
        />
      )}
    </div>
  )
}
