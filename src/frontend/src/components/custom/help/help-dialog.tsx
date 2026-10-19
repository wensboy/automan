import * as React from "react"
import { ChevronRight } from "lucide-react"
import { toast } from "sonner"

import MarkChunk from "@/components/custom/share/mark-chunk"
import VersionSwitcher from "@/components/custom/help/version-switcher"
import SearchForm from "@/components/custom/help/search-form"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  SidebarContent,
  SidebarGroup,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  SidebarProvider,
} from "@/components/ui/sidebar"
import { useStatic } from "@/components/custom/share/provider/static-provider"
import { Doc as LoadDoc, Meta as LoadMeta } from "../../../../bindings/github.com/wensboy/automan/backend/api/web/helpservice"
import type { Doc, Meta } from "../../../../bindings/github.com/wensboy/automan/backend/service/help/models"

/**
 * 帮助弹窗: 左侧目录来自 data/docs/meta.json 的 toc, 右侧是 MarkChunk 渲染的 markdown。
 * 全部走 help 服务的绑定(离线嵌入文档); 网络获取(remote 三个 uri)当前不实现。
 */
export default function HelpDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { Value } = useStatic()
  const [meta, setMeta] = React.useState<Meta | null>(null)
  const [doc, setDoc] = React.useState<Doc | null>(null)
  const [docId, setDocId] = React.useState("")
  const [loading, setLoading] = React.useState(false)
  // 目录分组默认**全部展开**: 用受控 open(不依赖 defaultOpen 的挂载时机)
  const [openIds, setOpenIds] = React.useState<string[]>([])

  const loadDoc = React.useCallback(async (id: string) => {
    if (!id) return
    setLoading(true)
    try {
      setDoc(await LoadDoc(id))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error))
    } finally {
      setLoading(false)
    }
  }, [])

  // 打开时拉一次 meta(目录/版本); 默认选中第一篇
  React.useEffect(() => {
    if (!open) return
    let cancelled = false
    void (async () => {
      try {
        const loaded = await LoadMeta()
        if (cancelled) return
        setMeta(loaded)
        setOpenIds((loaded?.toc ?? []).map((item) => item.id))
        const first = loaded?.toc?.[0]?.id ?? ""
        setDocId(first)
        if (first) await loadDoc(first)
      } catch (error) {
        toast.error(error instanceof Error ? error.message : String(error))
      }
    })()
    return () => {
      cancelled = true
    }
  }, [open, loadDoc])

  const title = doc?.label || meta?.toc?.find((item) => item.id === docId)?.label || ""

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="overflow-hidden p-0 md:max-h-[560px] md:max-w-[760px] lg:max-w-[920px]"
      >
        <DialogTitle className="sr-only">{Value("sidebar.help.title")}</DialogTitle>
        <DialogDescription className="sr-only">{Value("sidebar.help.fallback")}</DialogDescription>
        <SidebarProvider className="h-[540px] min-h-0">
          <div className="hidden w-64 shrink-0 flex-col border-r bg-sidebar md:flex">
            <SidebarHeader>
              <VersionSwitcher versions={meta ? [meta.version] : []} defaultVersion={meta?.version ?? ""} />
              <SearchForm />
            </SidebarHeader>
            <SidebarContent className="gap-0">
              <SidebarGroup>
                <SidebarMenu>
                  {(meta?.toc ?? []).map((item) => (
                    <Collapsible
                      key={item.id}
                      asChild
                      className="group/collapsible"
                      open={openIds.includes(item.id)}
                      onOpenChange={(next) =>
                        setOpenIds((prev) =>
                          next ? [...prev, item.id] : prev.filter((id) => id !== item.id),
                        )
                      }
                    >
                      <SidebarMenuItem>
                        {/* 一级本身可点击(加载该篇), 同时点击也会展开/收起子项 */}
                        <CollapsibleTrigger asChild>
                          <SidebarMenuButton
                            isActive={item.id === docId}
                            onClick={() => {
                              setDocId(item.id)
                              void loadDoc(item.id)
                            }}
                          >
                            <span>{item.label}</span>
                            <ChevronRight className="ml-auto transition-transform group-data-[state=open]/collapsible:rotate-90" />
                          </SidebarMenuButton>
                        </CollapsibleTrigger>
                        {(item.sub ?? []).length > 0 ? (
                          <CollapsibleContent>
                            {/* 容器不再横向内缩: 子项点击宽度与一级一致; 层级靠按钮左内边距表达 */}
                            <SidebarMenuSub className="mx-0 gap-0.5 border-l-0 px-0">
                              {(item.sub ?? []).map((leaf) => (
                                <SidebarMenuSubItem key={leaf.id}>
                                  <SidebarMenuSubButton
                                    asChild
                                    isActive={leaf.id === docId}
                                    className="h-8 w-full -translate-x-0 pl-6"
                                  >
                                    <button
                                      type="button"
                                      onClick={() => {
                                        setDocId(leaf.id)
                                        void loadDoc(leaf.id)
                                      }}
                                    >
                                      {leaf.label}
                                    </button>
                                  </SidebarMenuSubButton>
                                </SidebarMenuSubItem>
                              ))}
                            </SidebarMenuSub>
                          </CollapsibleContent>
                        ) : null}
                      </SidebarMenuItem>
                    </Collapsible>
                  ))}
                </SidebarMenu>
              </SidebarGroup>
            </SidebarContent>
          </div>
          <main className="flex h-[540px] flex-1 flex-col overflow-hidden">
            <header className="flex h-14 shrink-0 items-center gap-2 border-b">
              <div className="flex items-center gap-2 px-4">
                <Breadcrumb>
                  <BreadcrumbList>
                    <BreadcrumbItem className="hidden md:block">
                      <BreadcrumbLink href="#">{Value("sidebar.help.title")}</BreadcrumbLink>
                    </BreadcrumbItem>
                    <BreadcrumbSeparator className="hidden md:block" />
                    <BreadcrumbItem>
                      <BreadcrumbPage>{title}</BreadcrumbPage>
                    </BreadcrumbItem>
                  </BreadcrumbList>
                </Breadcrumb>
              </div>
            </header>
            <div className="flex min-h-0 flex-1 flex-col p-4">
              <MarkChunk
                className="min-h-0 flex-1"
                value={loading ? "" : (doc?.content ?? "")}
              />
            </div>
          </main>
        </SidebarProvider>
      </DialogContent>
    </Dialog>
  )
}
