import * as React from "react"
import { toast } from "sonner"

import ProxyFooter from "@/components/custom/proxy/footer"
import ProxySheet from "@/components/custom/proxy/proxy-sheet"
import ProxyTable from "@/components/custom/proxy/table"
import ProxyToolbar from "@/components/custom/proxy/toolbar"
import {
  Create,
  Delete,
  List,
  SetStatus,
  Start,
  Stop,
  Template,
  Update,
} from "../../../bindings/github.com/wensboy/automan/backend/api/web/proxyservice"
import type { ProxyAction, ProxyApplication } from "@/mock/proxies"
import { staticValue } from "@/components/custom/share/provider/static-provider"
import { toPayload, type Input, type Proxy, type ProxyStatus, type ProxyStatusFilter } from "@/components/custom/proxy/model"

/** 每页行数可选值 (页脚的选择器)。 */
const PAGE_SIZES = [5, 10, 20]

/** 搜索去抖: 输入停顿后再查询, 避免每次击键都打后端。 */
const SEARCH_DEBOUNCE_MS = 250

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

/**
 * 代理 (nav-Main: Proxy): header (搜索 + 工具按钮) + body (数据表与 footer 黏合)。
 *
 * 数据来自后端(v17 数据链路): 列表/搜索/分页走 List(服务端), 增删改走 Create/Update/Delete。
 * 行操作: edit 打开编辑 sheet; inspect 暂为 toast; delete 真删(逻辑删除); status/application 暂为 toast
 * (status 的真实变更属执行链路, application 与 proxy 解耦, 见 v17 §10/J 与 §11)。
 */
export default function Proxy() {
  const [rows, setRows] = React.useState<Proxy[]>([])
  const [total, setTotal] = React.useState(0)
  const [counts, setCounts] = React.useState<Record<string, number | undefined>>({})
  const [query, setQuery] = React.useState("")
  const [search, setSearch] = React.useState("")
  const [status, setStatus] = React.useState<ProxyStatusFilter>("all")
  const [page, setPage] = React.useState(1)
  const [rowsPerPage, setRowsPerPage] = React.useState(PAGE_SIZES[0])
  const [loading, setLoading] = React.useState(false)
  const [selectedIds, setSelectedIds] = React.useState<Set<string>>(new Set())
  const [rulesTemplate, setRulesTemplate] = React.useState<string | undefined>(undefined)
  const [sheetOpen, setSheetOpen] = React.useState(false)
  const [editRow, setEditRow] = React.useState<Proxy | null>(null)

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearch(query)
      setPage(1)
    }, SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [query])

  const reload = React.useCallback(async () => {
    setLoading(true)
    try {
      const result = await List({
        search,
        status: status === "all" ? "" : status,
        page,
        pageSize: rowsPerPage,
      })
      setRows(result?.items ?? [])
      setTotal(result?.total ?? 0)
      // 统计口径由后端给出: 只受搜索影响, 不受状态过滤收窄(否则选中某状态后其余计数恒为 0)
      setCounts(result?.counts ?? {})
    } catch (error) {
      toast.error(`Failed to load proxies: ${messageOf(error)}`)
    } finally {
      setLoading(false)
    }
  }, [search, status, page, rowsPerPage])

  React.useEffect(() => {
    void reload()
  }, [reload])

  // rules 模板来自后端(data/default/proxy.rules.json), 新建代理时作为编辑器初始内容。
  // 失败**不再静默**: 取不到就让用户看到原因, 否则只会表现为编辑器停在 {}。
  const loadRulesTemplate = React.useCallback(async (): Promise<string> => {
    try {
      const raw = await Template()
      if (raw && raw.trim() !== "") return raw
      toast.error("rules template is empty (data/default/proxy.rules.json)")
    } catch (error) {
      toast.error(`Template failed: ${messageOf(error)}`)
    }
    return ""
  }, [])

  React.useEffect(() => {
    void loadRulesTemplate().then((raw) => setRulesTemplate(raw || undefined))
  }, [loadRulesTemplate])

  const pageCount = Math.max(1, Math.ceil(total / rowsPerPage))
  const current = Math.min(page, pageCount)
  const rangeStart = total === 0 ? 0 : (current - 1) * rowsPerPage + 1
  const rangeEnd = Math.min(current * rowsPerPage, total)

  // 页脚统计: 后端按当前筛选结果给出(不受分页影响)
  const footerCounts = React.useMemo(
    () => ({
      running: counts.running ?? 0,
      exception: counts.exception ?? 0,
      error: counts.error ?? 0,
    }),
    [counts],
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
      const allSelected = rows.length > 0 && rows.every((row) => prev.has(row.id))
      for (const row of rows) {
        if (allSelected) {
          next.delete(row.id)
        } else {
          next.add(row.id)
        }
      }
      return next
    })
  }

  const openEdit = (row: Proxy) => {
    setEditRow(row)
    setSheetOpen(true)
  }

  const removeRow = async (row: Proxy) => {
    try {
      await Delete(row.id)
      toast.success(`${staticValue("contentArea.proxy.deleted") ?? ""}${row.name}`)
      setSelectedIds((prev) => {
        const next = new Set(prev)
        next.delete(row.id)
        return next
      })
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  // 编辑/新增: 落库后重新拉取当前页(总数与统计随之更新)
  const submitSheet = async (input: Input, id?: string) => {
    // 入库统一用紧凑 JSON(显示在编辑器里是缩进形式, 两者不同源: 显示层各自 pretty)
    const payload = toPayload(input)
    if (id) {
      await Update(id, payload)
    } else {
      await Create(payload)
    }
    toast.success(
      (id ? staticValue("contentArea.proxy.updated") : staticValue("contentArea.proxy.saved")) ?? "",
    )
    await reload()
  }

  // 启动: 参数以 sqlite 为准, 让步结果由后端写回并返回实际生效值
  const startSelected = async () => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) {
      toast.error(staticValue("contentArea.proxy.noSelection") ?? "")
      return
    }
    try {
      const result = await Start(ids)
      const started = result?.items?.length ?? 0
      const failed = result?.failed ?? []
      if (started > 0) {
        toast.success(`${staticValue("contentArea.proxy.started") ?? ""} (${started})`)
      }
      for (const item of failed) {
        toast.error(`${item.name}: ${item.reason}`)
      }
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  const stopSelected = async () => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) {
      toast.error(staticValue("contentArea.proxy.noSelection") ?? "")
      return
    }
    try {
      await Stop(ids)
      toast.success(staticValue("contentArea.proxy.stopped") ?? "")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  // 批量改状态: 后端只改"状态与目标不同"的行并返回实际条数。
  // 当前只落库 —— 真实启停(whistle/libretto)属执行链路, 见 v17 §11。
  const changeStatus = async (next: ProxyStatus, ids: string[]) => {
    if (ids.length === 0) {
      toast.error(staticValue("contentArea.proxy.noSelection") ?? "")
      return
    }
    try {
      const changed = await SetStatus(ids, next)
      toast.success(
        changed > 0
          ? `${staticValue("contentArea.proxy.statusChanged") ?? ""}${changed}`
          : (staticValue("contentArea.proxy.statusUnchanged") ?? ""),
      )
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  // 行菜单的状态动作: start/restart/reload -> running, stop -> stopped(只作用于该行)
  const applyStatus = (row: Proxy, action: ProxyAction) => {
    void changeStatus(action === "stop" ? "stopped" : "running", [row.id])
  }

  // application 与 proxy 解耦(v17 §10/J): 服务层不承载它, 因此这里只做 UI 反馈, 不再改行数据
  const applyApplication = (row: Proxy, application: ProxyApplication) => {
    toast.success(`${staticValue(`contentArea.proxy.applicationLabel.${application}`) as string} for ${row.name}`)
  }

  const refresh = async () => {
    await reload()
    toast.success(staticValue("contentArea.proxy.refreshed") ?? "")
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 p-4 pt-0">
      <ProxyToolbar
        query={query}
        onQueryChange={setQuery}
        status={status}
        onStatusChange={(value) => {
          setStatus(value)
          setPage(1)
        }}
        onAdd={() => {
          setEditRow(null)
          // 新建时用后端模板: 先确保取到再打开(取不到会 toast 出原因)
          void (async () => {
            const raw = rulesTemplate || (await loadRulesTemplate())
            if (raw) setRulesTemplate(raw)
            setSheetOpen(true)
          })()
        }}
        onRefresh={() => void refresh()}
        onClear={() => {
          setQuery("")
          setStatus("all")
          setPage(1)
        }}
        // 启动: 选中里状态非 running 的全部置为 running; 停止: 非 stopped 的全部置为 stopped
        onStart={() => void startSelected()}
        onStop={() => void stopSelected()}
      />
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border">
        <ProxyTable
          rows={rows}
          selectedIds={selectedIds}
          onToggleRow={toggleRow}
          onToggleAll={toggleAll}
          onEdit={openEdit}
          onInspect={(row) => toast(`${staticValue("contentArea.proxy.inspected") ?? ""}${row.name}`)}
          onDelete={(row) => void removeRow(row)}
          onStatus={applyStatus}
          onApplication={applyApplication}
        />
        <ProxyFooter
          rangeStart={rangeStart}
          rangeEnd={rangeEnd}
          total={total}
          counts={footerCounts}
          page={current}
          pageCount={pageCount}
          pageSizes={PAGE_SIZES}
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
        rulesTemplate={rulesTemplate}
        onSubmit={submitSheet}
      />
      {loading && (
        <span className="sr-only" role="status">
          Loading
        </span>
      )}
    </div>
  )
}
