import * as React from "react"
import { ChevronsUpDown, GalleryVerticalEnd, Plus, X } from "lucide-react"
import { toast } from "sonner"

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar"
import { staticValue } from "@/components/custom/share/provider/static-provider"
import { useSessionStore } from "@/store/session"
import { useWorkspaceStore } from "@/store/workspace"

/**
 * 工作区切换(替代原 team switcher):
 *   · 触发器显示当前工作区, 展开后列出全部工作区(点击切换);
 *   · 列表项 hover 时右侧出现删除按钮(阻止冒泡, 不会误触发切换);
 *   · 底部 "Add workspace" -> **Dialog** 输入名称后创建并切换。
 *
 * 为什么用 Dialog 而不是 Popover: 弹层挂在 DropdownMenu 里会随菜单卸载/被 outside 交互关掉;
 * Dialog 独立挂载在根节点, 不受下拉生命周期影响, 输入框焦点也更稳定。
 */
export default function WorkspaceSwitcher() {
  const { isMobile } = useSidebar()
  const { items, currentId, load, create, remove, select, reset } = useWorkspaceStore()
  const [addOpen, setAddOpen] = React.useState(false)
  const [name, setName] = React.useState("")
  const [pending, setPending] = React.useState(false)

  // 工作区列表**不缓存**: 用户变化时先清空再实时拉取, 避免跨用户显示
  const userId = useSessionStore((state) => state.user?.id ?? "")
  React.useEffect(() => {
    // 未登录只清空, 不发请求(避免登录页弹出 "requires a signed-in user")
    if (!userId) {
      reset()
      return
    }
    reset()
    void load().catch((error: unknown) => {
      toast.error(error instanceof Error ? error.message : String(error))
    })
  }, [userId, load, reset])

  // 当前工作区只认后端返回的 currentId: **不回落第一个**
  // (否则重启后展示一个"看起来选中"的工作区, 但后端并没有当前选择 -> 资源实际为空)
  const current = items.find((item) => item.id === currentId)

  const handleCreate = async () => {
    const trimmed = name.trim()
    if (!trimmed || pending) return
    setPending(true)
    try {
      await create(trimmed)
      setName("")
      setAddOpen(false)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error))
    } finally {
      setPending(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await remove(id)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error))
    }
  }

  return (
    <>
      <SidebarMenu>
        <SidebarMenuItem>
          <DropdownMenu>
            <SidebarMenuButton
              asChild
              size="lg"
              className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <DropdownMenuTrigger>
                <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                  <GalleryVerticalEnd className="size-4" />
                </div>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-medium">
                    {current?.name ?? staticValue("sidebar.workspace.empty")}
                  </span>
                  <span className="truncate text-xs text-muted-foreground">
                    {staticValue("sidebar.workspace.subtitle")}
                  </span>
                </div>
                <ChevronsUpDown className="ml-auto" />
              </DropdownMenuTrigger>
            </SidebarMenuButton>
            <DropdownMenuContent
              className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
              align="start"
              side={isMobile ? "bottom" : "right"}
              sideOffset={4}
              onCloseAutoFocus={(event) => event.preventDefault()}
            >
              <DropdownMenuLabel className="text-xs text-muted-foreground">
                {staticValue("sidebar.workspace.label")}
              </DropdownMenuLabel>
              {items.map((item) => (
                <DropdownMenuItem
                  key={item.id}
                  className="group/ws gap-2 p-2"
                  onSelect={() => {
                    void select(item.id).catch((error: unknown) => {
                      toast.error(error instanceof Error ? error.message : String(error))
                    })
                  }}
                >
                  <div className="flex size-6 items-center justify-center rounded-md border">
                    <GalleryVerticalEnd className="size-3.5 shrink-0" />
                  </div>
                  <span className="truncate">{item.name}</span>
                  <button
                    type="button"
                    aria-label={staticValue("sidebar.workspace.delete")}
                    className="ml-auto hidden size-5 items-center justify-center rounded hover:bg-destructive/10 hover:text-destructive group-hover/ws:flex"
                    onClick={(event) => {
                      event.preventDefault()
                      event.stopPropagation()
                      void handleDelete(item.id)
                    }}
                  >
                    <X className="size-3.5" />
                  </button>
                </DropdownMenuItem>
              ))}
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="gap-2 p-2"
                onSelect={(event) => {
                  event.preventDefault()
                  // 菜单关闭后再开 Dialog, 避免同一次交互里两个浮层互相打断
                  window.setTimeout(() => setAddOpen(true), 0)
                }}
              >
                <div className="flex size-6 items-center justify-center rounded-md border bg-transparent">
                  <Plus className="size-4" />
                </div>
                <span className="font-medium text-muted-foreground">
                  {staticValue("sidebar.workspace.add")}
                </span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </SidebarMenuItem>
      </SidebarMenu>

      <Dialog
        open={addOpen}
        onOpenChange={(open) => {
          setAddOpen(open)
          if (!open) setName("")
        }}
      >
        <DialogContent className="sm:max-w-md" showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>{staticValue("sidebar.workspace.add")}</DialogTitle>
          </DialogHeader>
          <Input
            autoFocus
            value={name}
            placeholder={staticValue("sidebar.workspace.placeholder") ?? ""}
            onChange={(event) => setName(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") void handleCreate()
            }}
          />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setAddOpen(false)}>
              {staticValue("sidebar.workspace.cancel")}
            </Button>
            <Button type="button" disabled={pending || name.trim() === ""} onClick={() => void handleCreate()}>
              {staticValue("sidebar.workspace.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
