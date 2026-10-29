import * as React from "react"
import { CircleAlertIcon } from "lucide-react"

import Combobox from "@/components/custom/share/combobox"
import EmptyState from "@/components/custom/share/empty-state"
import FileTree, { type FileTreeContext } from "@/components/custom/share/file-tree"
import { useStatic } from "@/components/custom/share/provider/static-provider"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import {
  createEnvDir,
  createEnvFile,
  deleteEnvDir,
  deleteEnvFile,
  loadEnvTree,
  prettyJSON,
  readEnvFile,
  refreshEnvFile,
  renameEnvPath,
  saveEnvFile,
  type EnvDocument,
  type EnvNode,
} from "@/lib/environment"
import { resolveIcon } from "@/lib/icons"
import { notify } from "@/lib/notice"

/**
 * Home > Environment(见 docs/design/v25.md §19 与 docs/design/v26.md)。
 *
 * 数据来源全部为后端 environments 服务:
 *   · 侧栏树 = Tree(); list = Read().entries(只读); file = Read().content(本地编辑, 尚未保存)
 *   · 右键动作直接调用 CreateDir/CreateFile/DeleteDir/DeleteFile/Rename/Refresh
 *   · 错误统一用 EmptyState(告警图标) 呈现, 同时 notify 一次
 */

type MenuItem = { id: string; icon: string; label: string }

type DialogState = {
  kind: "newFile" | "newFolder" | "rename" | "delete"
  parentPath: string
  target?: EnvNode
}

function flattenFiles(nodes: EnvNode[]): EnvNode[] {
  return nodes.flatMap((node) => (node.type === "file" ? [node] : node.children ? flattenFiles(node.children) : []))
}

function joinPath(parent: string, name: string): string {
  return parent ? `${parent}/${name}` : name
}

function fileWithSuffix(name: string): string {
  return name.toLowerCase().endsWith(".json") ? name : `${name}.json`
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

export default function EnvironmentView() {
  const { Value } = useStatic()
  const text = (path: string, fallback: string): string => {
    const value = Value(path)
    return typeof value === "string" ? value : fallback
  }

  const views = React.useMemo(() => {
    const items = Value("contentArea.home.environment.views")
    return Array.isArray(items) ? items : []
  }, [Value])

  const menus = React.useMemo(() => {
    const raw = Value("contentArea.home.environment.menus")
    const map: Record<string, MenuItem[]> = { dir: [], file: [], blank: [] }
    if (raw && typeof raw === "object" && !Array.isArray(raw)) {
      for (const key of Object.keys(map)) {
        const items = (raw as Record<string, unknown>)[key]
        map[key] = Array.isArray(items) ? (items as MenuItem[]) : []
      }
    }
    return map
  }, [Value])

  const [nodes, setNodes] = React.useState<EnvNode[]>([])
  const [treeError, setTreeError] = React.useState("")
  const [selectedPath, setSelectedPath] = React.useState<string | undefined>()
  const [revealId, setRevealId] = React.useState<string | undefined>()
  const [view, setView] = React.useState("list")
  const [doc, setDoc] = React.useState<EnvDocument | null>(null)
  const [docError, setDocError] = React.useState("")
  const [editor, setEditor] = React.useState("")
  const [context, setContext] = React.useState<FileTreeContext | null>(null)
  const [dialog, setDialog] = React.useState<DialogState | null>(null)
  const [dialogName, setDialogName] = React.useState("")
  const [dialogError, setDialogError] = React.useState("")
  const [saving, setSaving] = React.useState(false)

  const loadTree = React.useCallback(async () => {
    try {
      const tree = await loadEnvTree()
      setNodes(tree)
      setTreeError("")
    } catch (error) {
      setTreeError(messageOf(error))
    }
  }, [])

  React.useEffect(() => {
    void loadTree()
  }, [loadTree])

  const selectedNode = React.useMemo(() => {
    const walk = (items: EnvNode[]): EnvNode | undefined => {
      for (const node of items) {
        if (node.id === selectedPath) return node
        if (node.children) {
          const found = walk(node.children)
          if (found) return found
        }
      }
      return undefined
    }
    return walk(nodes)
  }, [nodes, selectedPath])

  // 仅选中**文件**时读取内容; 目录/清空 -> 不加载内容
  React.useEffect(() => {
    if (!selectedPath || selectedNode?.type !== "file") {
      setDoc(null)
      setDocError("")
      setEditor("")
      return
    }
    let alive = true
    void (async () => {
      try {
        const next = await readEnvFile(selectedPath)
        if (!alive) return
        setDoc(next)
        setEditor(prettyJSON(next.content))
        setDocError("")
      } catch (error) {
        if (!alive) return
        setDoc(null)
        setDocError(messageOf(error))
      }
    })()
    return () => {
      alive = false
    }
  }, [selectedPath, selectedNode])

  const files = React.useMemo(() => flattenFiles(nodes), [nodes])
  const selectedName = React.useMemo(() => {
    const walk = (items: EnvNode[]): string | undefined => {
      for (const node of items) {
        if (node.id === selectedPath) return node.name
        if (node.children) {
          const found = walk(node.children)
          if (found) return found
        }
      }
      return undefined
    }
    return walk(nodes)
  }, [nodes, selectedPath])

  React.useEffect(() => {
    if (!context) return
    const close = () => setContext(null)
    window.addEventListener("click", close)
    window.addEventListener("keydown", close)
    return () => {
      window.removeEventListener("click", close)
      window.removeEventListener("keydown", close)
    }
  }, [context])

  const SaveIcon = resolveIcon("save")
  const dirty = !!doc && editor !== prettyJSON(doc.content)

  const saveDocument = async () => {
    if (!doc || saving) return
    setSaving(true)
    try {
      const next = await saveEnvFile(doc.path, editor)
      setDoc(next)
      setEditor(prettyJSON(next.content))
      setDocError("")
    } catch (error) {
      notify("error", "environments", text("contentArea.home.environment.notify.saveFailed", "Save failed"), messageOf(error))
    } finally {
      setSaving(false)
    }
  }

  const menuItems = context ? menus[context.type === "dir" ? "dir" : context.type === "file" ? "file" : "blank"] : []

  const openCreate = (kind: "newFile" | "newFolder", parentPath: string) => {
    setDialog({ kind, parentPath })
    setDialogName("")
    setDialogError("")
  }

  const openRename = (node: EnvNode) => {
    const parentPath = node.id.includes("/") ? node.id.slice(0, node.id.lastIndexOf("/")) : ""
    setDialog({ kind: "rename", parentPath, target: node })
    setDialogName(node.name)
    setDialogError("")
  }

  const submitDialog = async () => {
    if (!dialog) return
    if (dialog.kind === "delete" && dialog.target) {
      try {
        const path = dialog.target.id
        if (dialog.target.type === "dir") await deleteEnvDir(path)
        else await deleteEnvFile(path)
        await loadTree()
        if (selectedPath && (selectedPath === path || selectedPath.startsWith(`${path}/`))) {
          setSelectedPath(undefined)
        }
        setDialog(null)
      } catch (error) {
        setDialogError(messageOf(error))
      }
      return
    }
    const name = dialogName.trim()
    if (!name) {
      setDialogError(text("contentArea.home.environment.dialog.nameRequired", "Name is required"))
      return
    }
    try {
      if (dialog.kind === "rename" && dialog.target) {
        const target = dialog.target.id
        const nextName = dialog.target.type === "file" ? fileWithSuffix(name) : name
        const to = joinPath(dialog.parentPath, nextName)
        await renameEnvPath(target, to)
        await loadTree()
        setSelectedPath(to)
        setRevealId(to)
      } else {
        const fileName = dialog.kind === "newFile" ? fileWithSuffix(name) : name
        const path = joinPath(dialog.parentPath, fileName)
        if (dialog.kind === "newFile") await createEnvFile(path)
        else await createEnvDir(path)
        await loadTree()
        setSelectedPath(path)
        setRevealId(path)
      }
      setDialog(null)
    } catch (error) {
      setDialogError(messageOf(error))
    }
  }

  const handleMenu = async (item: MenuItem) => {
    const target = context
    setContext(null)
    if (!target) return

    try {
      if (item.id === "newFile" || item.id === "newFolder") {
        const parentPath = target.type === "dir" && target.node ? target.node.id : ""
        openCreate(item.id === "newFile" ? "newFile" : "newFolder", parentPath)
        return
      }
      if (item.id === "rename" && target.node) {
        openRename(target.node)
        return
      }
      if (item.id === "delete" && target.node) {
        setDialog({ kind: "delete", parentPath: "", target: target.node })
        setDialogName("")
        setDialogError("")
        return
      }
      if (item.id === "refresh") {
        if (target.type === "file" && target.node) {
          const next = await refreshEnvFile(target.node.id)
          setDoc(next)
          setEditor(prettyJSON(next.content))
          setDocError("")
        } else {
          await loadTree()
          if (target.type === "dir" && target.node) setRevealId(target.node.id)
        }
      }
    } catch (error) {
      notify("error", "environments", text("contentArea.home.environment.notify.operationFailed", "Operation failed"), messageOf(error))
    }
  }

  return (
    <div className="flex min-h-0 flex-1">
      {/* 左: file tree + 底部搜索 */}
      <aside className="flex w-64 shrink-0 flex-col border-r">
        <div className="min-h-0 flex-1 overflow-auto px-1 py-1">
          {treeError ? (
            <div className="px-2 py-2 text-xs text-destructive">{treeError}</div>
          ) : (
            <FileTree
              nodes={nodes}
              selectedId={selectedPath}
              revealId={revealId}
              onSelect={(node) => setSelectedPath(node.id)}
              onContextMenu={setContext}
            />
          )}
        </div>

        <div className="shrink-0 border-t p-1">
          <Combobox
            items={files.map((file) => ({ value: file.id, label: file.id }))}
            onChange={(id) => {
              setSelectedPath(id)
              setRevealId(id)
            }}
          />
        </div>
      </aside>

      {/* 右: header + 视图主体 */}
      <section className="flex min-h-0 min-w-0 flex-1 flex-col">
        <header className="flex h-9 shrink-0 items-center gap-2 border-b px-2">
          <span className="min-w-0 flex-1 truncate text-xs" title={selectedName ?? ""}>
            {selectedName ?? text("contentArea.home.environment.itemPlaceholder", "No item selected")}
          </span>
          {dirty ? (
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              title={text("contentArea.home.environment.saveAria", "Save changes")}
              aria-label={text("contentArea.home.environment.saveAria", "Save changes")}
              disabled={saving}
              onClick={() => void saveDocument()}
            >
              <SaveIcon className="size-4" />
            </Button>
          ) : null}
          <div className="flex items-center gap-1">
            {views.map((item) => {
              const Icon = resolveIcon(String(item.icon))
              const active = String(item.id) === view
              return (
                <Button
                  key={String(item.id)}
                  variant={active ? "secondary" : "ghost"}
                  size="icon"
                  className="size-7"
                  title={String(item.label)}
                  aria-label={String(item.label)}
                  onClick={() => setView(String(item.id))}
                >
                  <Icon className="size-4" />
                </Button>
              )
            })}
          </div>
        </header>

        {view === "list" ? (
          docError ? (
            <div className="flex min-h-0 flex-1 flex-col">
              <EmptyState
                icon={<CircleAlertIcon className="text-destructive" />}
                title={text("contentArea.home.environment.list.errorTitle", "Invalid env file")}
                description={docError}
              />
            </div>
          ) : !doc ? (
            <div className="flex min-h-0 flex-1 flex-col">
              <EmptyState
                title={text("contentArea.home.environment.empty.title", "Environment")}
                description={text("contentArea.home.environment.empty.description", "This feature is not implemented yet.")}
              />
            </div>
          ) : (
            <div className="flex min-h-0 flex-1 flex-col">
              <div className="grid shrink-0 grid-cols-[140px_1fr_1fr] border-b text-xs">
                <div className="border-r px-2 py-1.5 text-muted-foreground">
                  {text("contentArea.home.environment.list.columns.type", "Type")}
                </div>
                <div className="border-r px-2 py-1.5 text-muted-foreground">
                  {text("contentArea.home.environment.list.columns.key", "Env key")}
                </div>
                <div className="px-2 py-1.5 text-muted-foreground">
                  {text("contentArea.home.environment.list.columns.value", "Env value")}
                </div>
              </div>
              <div className="min-h-0 flex-1 overflow-auto text-xs">
                {doc.entries.map((row) => (
                  <div key={row.key} className="grid grid-cols-[140px_1fr_1fr] border-b border-border/50 last:border-0">
                    <div className="border-r px-2 py-1.5 text-muted-foreground">{row.type}</div>
                    <div className="truncate border-r px-2 py-1.5" title={row.key}>{row.key}</div>
                    <div className="truncate px-2 py-1.5" title={row.value}>{row.value}</div>
                  </div>
                ))}
              </div>
            </div>
          )
        ) : (
          <div className="flex min-h-0 flex-1 flex-col">
            {docError ? (
              <EmptyState
                icon={<CircleAlertIcon className="text-destructive" />}
                title={text("contentArea.home.environment.list.errorTitle", "Invalid env file")}
                description={docError}
              />
            ) : !doc ? (
              <EmptyState
                title={text("contentArea.home.environment.empty.title", "Environment")}
                description={text("contentArea.home.environment.empty.description", "Select a file to view its environment.")}
              />
            ) : (
              <textarea
                value={editor}
                spellCheck={false}
                className="h-full w-full flex-1 resize-none border-0 bg-transparent p-2 font-mono text-xs leading-relaxed outline-none"
                onChange={(event) => setEditor(event.target.value)}
              />
            )}
          </div>
        )}
      </section>

      {/* 右键菜单 */}
      {context ? (
        <div
          className="fixed z-50 w-44 rounded-md border bg-popover p-1 text-xs shadow-md"
          style={{ left: context.x, top: context.y }}
          onClick={(event) => event.stopPropagation()}
        >
          {menuItems.map((item) => {
            const Icon = resolveIcon(item.icon)
            return (
              <button
                key={item.id}
                type="button"
                className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left hover:bg-muted"
                onClick={() => void handleMenu(item)}
              >
                <Icon className="size-3.5 text-muted-foreground" />
                <span>{item.label}</span>
              </button>
            )
          })}
        </div>
      ) : null}

      {/* 命名 / 重命名 */}
      <Dialog open={!!dialog} onOpenChange={(next) => (!next ? setDialog(null) : undefined)}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>
              {dialog
                ? dialog.kind === "delete"
                  ? text("contentArea.home.environment.dialog.deleteTitle", "Delete")
                  : dialog.kind === "rename"
                    ? text("contentArea.home.environment.dialog.rename", "Rename")
                    : dialog.kind === "newFolder"
                      ? text("contentArea.home.environment.dialog.newFolder", "New folder")
                      : text("contentArea.home.environment.dialog.newFile", "New file")
                : ""}
            </DialogTitle>
          </DialogHeader>
          <div className="grid gap-2">
            {dialog?.kind === "delete" ? (
              <p className="text-xs text-muted-foreground">
                {text("contentArea.home.environment.dialog.deleteMessage", "Delete this item? This cannot be undone.").replace(
                  "{name}",
                  dialog.target?.name ?? "",
                )}
              </p>
            ) : (
            <Input
              autoFocus
              value={dialogName}
              placeholder={text("contentArea.home.environment.dialog.namePlaceholder", "Name")}
              onChange={(event) => {
                setDialogName(event.target.value)
                setDialogError("")
              }}
              onKeyDown={(event) => {
                if (event.key === "Enter") void submitDialog()
              }}
            />
            )}
            {dialogError ? <p className="text-xs text-destructive">{dialogError}</p> : null}
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialog(null)}>
              {text("contentArea.home.environment.dialog.cancel", "Cancel")}
            </Button>
            <Button variant={dialog?.kind === "delete" ? "destructive" : "default"} onClick={() => void submitDialog()}>
              {dialog?.kind === "delete"
                ? text("contentArea.home.environment.dialog.deleteConfirm", "Delete")
                : dialog?.kind === "rename"
                  ? text("contentArea.home.environment.dialog.save", "Rename")
                  : text("contentArea.home.environment.dialog.create", "Create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
