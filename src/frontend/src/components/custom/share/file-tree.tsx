import * as React from "react"

import type { FileNode } from "@/mock/environment"
import { cn } from "cn"
import { ChevronRightIcon } from "lucide-react"

/**
 * 通用文件树(见 docs/design/v25.md §19)。
 *
 * 抽到 share/ 便于后续复用; 只负责结构与交互, 不含数据与菜单内容:
 *   · 目录可展开/折叠(受控 selectedId + 内部 expanded);
 *   · 右键分为 dir / file / blank 三类, 由 onContextMenu 交给调用方决定菜单;
 *   · revealId 用于外部(搜索跳转)展开并定位到某个节点。
 */
export type FileTreeContextType = "dir" | "file" | "blank"

export type FileTreeContext = {
  type: FileTreeContextType
  node?: FileNode
  x: number
  y: number
}

type FileTreeProps = {
  nodes: FileNode[]
  selectedId?: string
  /** 搜索跳转: 展开祖先并选中 */
  revealId?: string
  onSelect?: (node: FileNode) => void
  onContextMenu?: (context: FileTreeContext) => void
  className?: string
}

type FlatNode = {
  node: FileNode
  depth: number
}

function flatten(nodes: FileNode[], expanded: Set<string>, depth = 0): FlatNode[] {
  const result: FlatNode[] = []
  for (const node of nodes) {
    result.push({ node, depth })
    if (node.type === "dir" && node.children?.length && expanded.has(node.id)) {
      result.push(...flatten(node.children, expanded, depth + 1))
    }
  }
  return result
}

/** 找到目标节点及其祖先链(dirs), 用于 reveal。 */
function ancestorsOf(nodes: FileNode[], id: string, trail: string[] = []): string[] | null {
  for (const node of nodes) {
    if (node.id === id) return trail
    if (node.type === "dir" && node.children?.length) {
      const found = ancestorsOf(node.children, id, [...trail, node.id])
      if (found) return found
    }
  }
  return null
}

export default function FileTree({ nodes, selectedId, revealId, onSelect, onContextMenu, className }: FileTreeProps) {
  const [expanded, setExpanded] = React.useState<Set<string>>(() => new Set(nodes.filter((n) => n.type === "dir").map((n) => n.id)))

  React.useEffect(() => {
    if (!revealId) return
    const trail = ancestorsOf(nodes, revealId)
    if (!trail) return
    setExpanded((prev) => {
      const next = new Set(prev)
      for (const id of trail) next.add(id)
      return next
    })
  }, [nodes, revealId])

  const rows = React.useMemo(() => flatten(nodes, expanded), [nodes, expanded])

  const handleContext = (event: React.MouseEvent, type: FileTreeContextType, node?: FileNode) => {
    if (!onContextMenu) return
    event.preventDefault()
    event.stopPropagation()
    onContextMenu({ type, node, x: event.clientX, y: event.clientY })
  }

  return (
    <div
      className={cn("relative h-full w-full select-none", className)}
      onContextMenu={(event) => {
        // 仅空白处触发; 行内已 stopPropagation
        if (event.target === event.currentTarget) handleContext(event, "blank")
      }}
    >
      {rows.map(({ node, depth }) => {
        const isDir = node.type === "dir"
        const isOpen = expanded.has(node.id)
        const selected = node.id === selectedId
        return (
          <div
            key={node.id}
            role="treeitem"
            aria-expanded={isDir ? isOpen : undefined}
            tabIndex={0}
            onClick={() => {
              if (isDir) {
                setExpanded((prev) => {
                  const next = new Set(prev)
                  if (next.has(node.id)) next.delete(node.id)
                  else next.add(node.id)
                  return next
                })
              }
              onSelect?.(node)
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter") onSelect?.(node)
            }}
            onContextMenu={(event) => handleContext(event, isDir ? "dir" : "file", node)}
            className={cn(
              "flex h-7 cursor-default items-center gap-1 rounded-sm pr-2 text-xs",
              "hover:bg-muted/60",
              selected && "bg-muted text-foreground",
            )}
            style={{ paddingLeft: depth * 12 + 6 }}
          >
            <ChevronRightIcon
              className={cn(
                "size-3.5 shrink-0 text-muted-foreground transition-transform",
                isDir ? (isOpen ? "rotate-90" : "") : "opacity-0",
              )}
            />
            <span className={cn("truncate", isDir ? "text-foreground" : "text-muted-foreground")}>{node.name}</span>
          </div>
        )
      })}

      {/* 空白区域: 撑满剩余高度, 便于空白右键 */}
      <div className="h-8 w-full" onContextMenu={(event) => handleContext(event, "blank")} />
    </div>
  )
}
