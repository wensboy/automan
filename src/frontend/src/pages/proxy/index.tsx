import * as React from "react"

import ProxyFooter from "@/components/custom/proxy/footer"
import ProxySheet from "@/components/custom/proxy/proxy-sheet"
import ProxyTable from "@/components/custom/proxy/table"
import ProxyToolbar from "@/components/custom/proxy/toolbar"
import {
  Create,
  Delete,
  DeleteMany,
  List,
  Start,
  Stop,
  Template,
  Update,
} from "../../../bindings/github.com/wensboy/automan/backend/api/web/proxyservice"
import { staticValue } from "@/components/custom/share/provider/static-provider"
import { useWorkspaceStore } from "@/store/workspace"
import { notify } from "@/lib/notice"
import {
  toPayload,
  type Input,
  type Proxy,
  type ProxyAction,
  type ProxyStatusFilter,
} from "@/components/custom/proxy/model"

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
      notify("error", "proxy", `Failed to load proxies: ${messageOf(error)}`)
    } finally {
      setLoading(false)
    }
  }, [search, status, page, rowsPerPage])

  // 工作区切换后重新拉取(代理按工作区隔离)
  const workspaceId = useWorkspaceStore((state) => state.currentId)
  React.useEffect(() => {
    void reload()
  }, [reload, workspaceId])

  // rules 模板来自后端(data/default/proxy.rules.json), 新建代理时作为编辑器初始内容。
  // 失败**不再静默**: 取不到就让用户看到原因, 否则只会表现为编辑器停在 {}。
  const loadRulesTemplate = React.useCallback(async (): Promise<string> => {
    try {
      const raw = await Template()
      if (raw && raw.trim() !== "") return raw
      notify("error", "proxy", "rules template is empty (data/default/proxy.rules.json)")
    } catch (error) {
      notify("error", "proxy", `Template failed: ${messageOf(error)}`)
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

  const deleteSelected = async () => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) {
      notify("error", "proxy", staticValue("contentArea.proxy.noSelection") ?? "")
      return
    }
    try {
      const result = await DeleteMany(ids)
      const removed = new Set(result?.deleted ?? [])
      setSelectedIds((prev) => {
        const next = new Set(prev)
        for (const id of removed) next.delete(id)
        return next
      })
      // 跳过的明细由后端汇总为一条 warn notice, 前端不再重复提示
      await reload()
    } catch (error) {
      notify("error", "proxy", messageOf(error))
    }
  }

  const removeRow = async (row: Proxy) => {
    try {
      await Delete(row.id)
      setSelectedIds((prev) => {
        const next = new Set(prev)
        next.delete(row.id)
        return next
      })
      await reload()
    } catch (error) {
      notify("error", "proxy", messageOf(error))
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
    await reload()
  }

  // 启动: 参数以 sqlite 为准, 让步结果由后端写回并返回实际生效值
  const startIds = async (ids: string[]) => {
    if (ids.length === 0) {
      notify("error", "proxy", staticValue("contentArea.proxy.noSelection") ?? "")
      return
    }
    try {
      // 成功/逐条失败提示由后端 service 走 notice; 这里只负责刷新列表与兜底异常
      await Start(ids)
      await reload()
    } catch (error) {
      notify("error", "proxy", messageOf(error))
    }
  }

  const stopIds = async (ids: string[]) => {
    if (ids.length === 0) {
      notify("error", "proxy", staticValue("contentArea.proxy.noSelection") ?? "")
      return
    }
    try {
      await Stop(ids)
      await reload()
    } catch (error) {
      notify("error", "proxy", messageOf(error))
    }
  }

  // 行菜单的状态动作: **走真实执行链**(Start/Stop), 不再只写库。
  // 早先这里调 SetStatus(只改 status 列): 表现为"状态变了但代理没起来/没销毁"(见 v23 §22/§23)。
  const applyStatus = (row: Proxy, action: ProxyAction) => {
    void (async () => {
      if (action === "stop") {
        await stopIds([row.id])
        return
      }
      if (action === "restart") {
        // restart 语义: 先真正停止再启动(否则 Start 幂等, 等于什么都没做)
        await stopIds([row.id])
        await startIds([row.id])
        return
      }
      // start / reload: 统一走启动(start 幂等; reload 不额外热重载配置)
      await startIds([row.id])
    })()
  }

  const refresh = async () => {
    await reload()
    notify("success", "proxy", staticValue("contentArea.proxy.refreshed") ?? "")
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
        onDelete={() => void deleteSelected()}
        deleteDisabled={selectedIds.size === 0}
        // 启动: 选中里状态非 running 的全部置为 running; 停止: 非 stopped 的全部置为 stopped
        onStart={() => void startIds(Array.from(selectedIds))}
        onStop={() => void stopIds(Array.from(selectedIds))}
      />
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border">
        <ProxyTable
          rows={rows}
          selectedIds={selectedIds}
          onToggleRow={toggleRow}
          onToggleAll={toggleAll}
          onEdit={openEdit}
          onInspect={(row) =>
            notify("info", "proxy", `${staticValue("contentArea.proxy.inspected") ?? ""}${row.name}`)
          }
          onDelete={(row) => void removeRow(row)}
          onStatus={applyStatus}
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
          {staticValue("contentArea.shared.loading")}
        </span>
      )}
    </div>
  )
}
