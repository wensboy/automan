import * as React from "react"
import {
  CircleAlertIcon,
  DownloadIcon,
  ListIcon,
  PencilIcon,
  RefreshCwIcon,
  Trash2Icon,
  UploadIcon,
} from "lucide-react"
import { cn } from "cn"

import EmptyState from "@/components/custom/share/empty-state"
import { useStatic } from "@/components/custom/share/provider/static-provider"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
  deleteCertificate,
  exportCertificate,
  importCertificate,
  loadCertificates,
  pickCertificateFile,
  pickExportPath,
  readCertificate,
  saveCertificate,
  subscribeCertificates,
  syncCertificates,
  type Document as CertificateDocument,
  type Record as CertificateRecord,
} from "@/lib/certificate"

/**
 * Home > Certificates: 管理 whistle 全局证书目录(见 docs/design/v27.md)。
 *
 * 数据全部来自后端 certificate 服务(证书目录是唯一事实来源):
 *   · 列表 = List()(ID 升序, UI 倒序显示), 变化由 certificate:changed 事件驱动重载;
 *   · 同步 = Sync()(按证书目录校正记录), 导入/导出/删除走同名 API;
 *   · 右侧两态: 查看 = 顶层 [key, value] 表; 编辑 = 整块 JSON 编辑区, 切回查看时自动保存。
 */

/** 左侧列表: id / name(可伸缩) / type */
const rowGrid = "grid grid-cols-[48px_minmax(0,1fr)_64px] items-center text-xs"
/** 右侧文档: key / value(可伸缩) */
const entryGrid = "grid grid-cols-[minmax(0,160px)_minmax(0,1fr)] text-xs"

/** 文档中的一条字段; 对象/数组序列化为单行文本, 字符串原样显示。 */
type CertificateField = { key: string; value: string }

function fieldText(value: unknown): string {
  return typeof value === "string" ? value : JSON.stringify(value)
}

function parseFields(content: string): { ok: boolean; entries: CertificateField[] } {
  try {
    const document: unknown = JSON.parse(content)
    if (typeof document !== "object" || document === null || Array.isArray(document)) {
      return { ok: false, entries: [] }
    }
    // 所有字段都展示(空值也占一行, 便于看清结构)
    return { ok: true, entries: Object.entries(document).map(([key, value]) => ({ key, value: fieldText(value) })) }
  } catch {
    return { ok: false, entries: [] }
  }
}

export default function CertificatesView() {
  const { Value } = useStatic()
  const text = (key: string, fallback: string): string => {
    const value = Value(`contentArea.home.certificates.${key}`)
    return typeof value === "string" ? value : fallback
  }
  const label = (action: string): string => text(`actions.${action}`, action)

  const [records, setRecords] = React.useState<CertificateRecord[]>([])
  // 选中项按 certPath 记(后端每次同步会把 id 重排成 1..n), id 只从当前记录里派生
  const [selectedPath, setSelectedPath] = React.useState<string | null>(null)
  const [document, setDocument] = React.useState<CertificateDocument | null>(null)
  const [editing, setEditing] = React.useState(false)
  const [draft, setDraft] = React.useState("")
  const [deleteOpen, setDeleteOpen] = React.useState(false)

  // 操作结果(成功/失败)统一由后端服务投递 notice; 前端只处理本地状态, 不重复 toast
  const fail = (error: unknown) => console.error("[certificate]", error)

  const reload = React.useCallback(async () => {
    try {
      const snapshot = await loadCertificates()
      setRecords(snapshot.records ?? [])
    } catch (error) {
      fail(error)
    }
  }, [])

  React.useEffect(() => {
    void reload()
    return subscribeCertificates(() => void reload())
  }, [reload])

  // 选中项被外部删除(同步/删除后) -> 回到未选中态
  React.useEffect(() => {
    if (selectedPath !== null && !records.some((item) => item.certPath === selectedPath)) {
      setSelectedPath(null)
      setDocument(null)
      setEditing(false)
    }
  }, [records, selectedPath])

  const record = records.find((item) => item.certPath === selectedPath)
  // 倒序: ID 越大越新, 新建/导入的条目显示在顶部
  const ordered = React.useMemo(() => [...records].sort((left, right) => right.id - left.id), [records])
  const parsed = React.useMemo(() => parseFields(document?.content ?? ""), [document])

  const select = (item: CertificateRecord) => {
    setSelectedPath(item.certPath)
    setEditing(false)
    void (async () => {
      try {
        setDocument(await readCertificate(item.id))
      } catch (error) {
        fail(error)
      }
    })()
  }

  // 同步会重排 id: 用 certPath 重新定位选中项, 再按新的 id 重新读取详情
  const sync = () => {
    void (async () => {
      try {
        const snapshot = await syncCertificates()
        const list = snapshot.records ?? []
        setRecords(list)
        const current = list.find((item) => item.certPath === selectedPath)
        if (!current) {
          setSelectedPath(null)
          setDocument(null)
          setEditing(false)
          return
        }
        setDocument(await readCertificate(current.id))
      } catch (error) {
        fail(error)
      }
    })()
  }

  const importFile = () => {
    void (async () => {
      const path = await pickCertificateFile()
      if (!path) return
      try {
        const imported = await importCertificate(path)
        await reload()
        setSelectedPath(imported.record.certPath)
        setDocument(imported)
        setEditing(false)
      } catch (error) {
        fail(error)
      }
    })()
  }

  const exportCurrent = () => {
    if (!record) return
    void (async () => {
      const target = await pickExportPath(record)
      if (!target) return
      try {
        await exportCertificate(record.id, target)
      } catch (error) {
        fail(error)
      }
    })()
  }

  // 编辑 -> 查看: 自动保存; 查看 -> 编辑: 用后端返回的原文开草稿
  const toggleEditing = () => {
    if (!document) return
    if (!editing) {
      setDraft(document.content)
      setEditing(true)
      return
    }
    void (async () => {
      try {
        setDocument(await saveCertificate(document.record.id, draft))
        setEditing(false)
        await reload()
      } catch (error) {
        fail(error)
      }
    })()
  }

  // 删除边界: false = 只删记录(文件保留, 下次同步会重新登记), true = 连证书文件一起删
  const removeCurrent = (removeFile: boolean) => {
    if (!record) return
    void (async () => {
      try {
        await deleteCertificate(record.id, removeFile)
        setDeleteOpen(false)
        setSelectedPath(null)
        setDocument(null)
        setEditing(false)
        await reload()
      } catch (error) {
        fail(error)
      }
    })()
  }

  return (
    <div className="flex min-h-0 min-w-0 flex-1 gap-2 p-2">
      {/* 左: action header + table header + 列表 */}
      <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden rounded-md border">
        <header className="flex h-9 shrink-0 items-center gap-1 border-b px-2">
          <Button
            type="button"
            variant="outline"
            size="icon-xs"
            title={label("sync")}
            aria-label={label("sync")}
            onClick={sync}
          >
            <RefreshCwIcon />
          </Button>
          <Button
            type="button"
            variant="outline"
            size="icon-xs"
            title={label("import")}
            aria-label={label("import")}
            onClick={importFile}
          >
            <UploadIcon />
          </Button>
        </header>

        <div className="min-h-0 flex-1 overflow-auto">
          <div className={cn(rowGrid, "sticky top-0 z-10 border-b bg-background text-muted-foreground")}>
            <span className="px-2 py-1.5">{text("columns.id", "ID")}</span>
            <span className="px-2 py-1.5">{text("columns.name", "Name")}</span>
            <span className="px-2 py-1.5">{text("columns.type", "Type")}</span>
          </div>

          {ordered.map((item) => {
            const active = item.certPath === selectedPath
            return (
              <button
                key={item.id}
                type="button"
                aria-pressed={active}
                onClick={() => select(item)}
                className={cn(
                  rowGrid,
                  "w-full border-b border-border/50 text-left last:border-0 hover:bg-muted/40",
                  active && "bg-muted",
                )}
              >
                <span className="px-2 py-1.5 tabular-nums text-muted-foreground">{item.id}</span>
                <span className="truncate px-2 py-1.5" title={item.name}>
                  {item.name}
                </span>
                <span className="px-2 py-1.5 text-muted-foreground">{item.type}</span>
              </button>
            )
          })}
        </div>
      </section>

      {/* 右: header(左标识 / 右动作) + 主体(查看 | 编辑) */}
      <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden rounded-md border">
        <header className="flex h-9 shrink-0 items-center gap-2 border-b px-2">
          <span className="min-w-0 flex-1 truncate text-xs" title={record?.name ?? ""}>
            {record?.name ?? text("noSelection", "No certificate selected")}
          </span>
          <div className="flex shrink-0 items-center gap-1">
            <Button
              type="button"
              variant={editing ? "secondary" : "outline"}
              size="icon-xs"
              aria-pressed={editing}
              title={label(editing ? "view" : "edit")}
              aria-label={label(editing ? "view" : "edit")}
              disabled={!document}
              onClick={toggleEditing}
            >
              {editing ? <ListIcon /> : <PencilIcon />}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="icon-xs"
              title={label("export")}
              aria-label={label("export")}
              disabled={!record}
              onClick={exportCurrent}
            >
              <DownloadIcon />
            </Button>
            <Button
              type="button"
              variant="outline"
              size="icon-xs"
              title={label("delete")}
              aria-label={label("delete")}
              disabled={!record}
              onClick={() => setDeleteOpen(true)}
            >
              <Trash2Icon />
            </Button>
          </div>
        </header>

        {!document ? (
          <EmptyState
            title={text("noSelection", "No certificate selected")}
            description={text("emptyDescription", "Select a certificate to inspect its details.")}
          />
        ) : editing ? (
          <textarea
            value={draft}
            spellCheck={false}
            aria-label={text("editorAria", "Certificate document")}
            onChange={(event) => setDraft(event.target.value)}
            className="min-h-0 w-full flex-1 resize-none overflow-auto border-0 bg-transparent p-2 font-mono text-xs leading-relaxed outline-none"
          />
        ) : !parsed.ok ? (
          <EmptyState
            icon={<CircleAlertIcon className="text-destructive" />}
            title={text("errorTitle", "Invalid certificate document")}
            description={text("errorDescription", "The document is not a valid JSON object.")}
          />
        ) : (
          <div className="flex min-h-0 flex-1 flex-col">
            <div className={cn(entryGrid, "shrink-0 border-b text-muted-foreground")}>
              <span className="border-r px-2 py-1.5">{text("columns.key", "Key")}</span>
              <span className="px-2 py-1.5">{text("columns.value", "Value")}</span>
            </div>
            <div className="min-h-0 flex-1 overflow-auto">
              {parsed.entries.map((entry) => (
                <div key={entry.key} className={cn(entryGrid, "border-b border-border/50 last:border-0")}>
                  <span className="truncate border-r px-2 py-1.5 text-muted-foreground" title={entry.key}>
                    {entry.key}
                  </span>
                  <span className="truncate px-2 py-1.5" title={entry.value}>
                    {entry.value}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </section>

      {/* 删除边界: 只删映射 / 连证书文件一起删 */}
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>{text("deleteDialog.title", "Delete certificate")}</DialogTitle>
          </DialogHeader>
          <p className="text-xs text-muted-foreground">
            {text("deleteDialog.message", "Delete \"{name}\"? Choose what to remove.").replace("{name}", record?.name ?? "")}
          </p>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeleteOpen(false)}>
              {text("deleteDialog.cancel", "Cancel")}
            </Button>
            <Button variant="outline" onClick={() => removeCurrent(false)}>
              {text("deleteDialog.recordOnly", "Record only")}
            </Button>
            <Button variant="outline" className="hover:text-destructive" onClick={() => removeCurrent(true)}>
              {text("deleteDialog.withFile", "Record and file")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
