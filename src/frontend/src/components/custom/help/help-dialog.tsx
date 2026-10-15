import * as React from "react"
import { ChevronRight } from "lucide-react"

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
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
} from "@/components/ui/sidebar"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/**
 * 帮助弹窗: 弹窗内嵌文档侧栏 (复刻 shadcn sidebar-02), 面向文档浏览。
 * 与设置弹窗同构, 但侧栏为可折叠分组式导航且不允许收起 (collapsible="none")。
 */
export default function HelpDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { Value } = useStatic()
  const [selected, setSelected] = React.useState((Value("sidebar.help.groups") ?? [])[0]?.items[0]?.title ?? "")

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="overflow-hidden p-0 md:max-h-[500px] md:max-w-[700px] lg:max-w-[800px]"
      >
        <DialogTitle className="sr-only">{Value("sidebar.help.title")}</DialogTitle>
        <DialogDescription className="sr-only">
          {Value("sidebar.help.fallback")}
        </DialogDescription>
        {/* 高度与右侧 main 一致; min-h-0 抵消 ui 的 min-h-svh */}
        <SidebarProvider className="h-[480px] min-h-0">
          {/*
            弹窗内的侧栏不用 ui 的 Sidebar 容器: 它为整页设计 —— fixed + h-svh 渲染侧栏本体,
            同时另起一个 gap 占位预留宽度, 于是"预留空间"和"实际渲染"是两套坐标,
            放进固定高度的弹窗里高度一改就错位/覆盖。这里改用普通 flex 列承载同样的组成件
            (SidebarHeader/SidebarContent/… 仍在 SidebarProvider 内), 几何完全显式。
          */}
          <div className="hidden w-64 shrink-0 flex-col border-r bg-sidebar md:flex">
            <SidebarHeader>
              <VersionSwitcher
                versions={Value("sidebar.help.versions") ?? []}
                defaultVersion={(Value("sidebar.help.versions") ?? [""])[0]}
              />
              <SearchForm />
            </SidebarHeader>
            <SidebarContent className="gap-0">
              {(Value("sidebar.help.groups") ?? []).map((group) => (
                <Collapsible
                  key={group.title}
                  title={group.title}
                  defaultOpen
                  className="group/collapsible"
                >
                  <SidebarGroup>
                    <SidebarGroupLabel
                      asChild
                      className="group/label text-sm text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                    >
                      <CollapsibleTrigger>
                        {group.title}{" "}
                        <ChevronRight className="ml-auto transition-transform group-data-[state=open]/collapsible:rotate-90" />
                      </CollapsibleTrigger>
                    </SidebarGroupLabel>
                    <CollapsibleContent>
                      <SidebarGroupContent>
                        <SidebarMenu>
                          {group.items.map((doc) => (
                            <SidebarMenuItem key={doc.title}>
                              <SidebarMenuButton
                                asChild
                                isActive={doc.title === selected}
                              >
                                <button
                                  type="button"
                                  onClick={() => setSelected(doc.title)}
                                >
                                  {doc.title}
                                </button>
                              </SidebarMenuButton>
                            </SidebarMenuItem>
                          ))}
                        </SidebarMenu>
                      </SidebarGroupContent>
                    </CollapsibleContent>
                  </SidebarGroup>
                </Collapsible>
              ))}
            </SidebarContent>
          </div>
          <main className="flex h-[480px] flex-1 flex-col overflow-hidden">
            <header className="flex h-16 shrink-0 items-center gap-2 transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-12">
              <div className="flex items-center gap-2 px-4">
                <Breadcrumb>
                  <BreadcrumbList>
                    <BreadcrumbItem className="hidden md:block">
                      <BreadcrumbLink href="#">{Value("sidebar.help.title")}</BreadcrumbLink>
                    </BreadcrumbItem>
                    <BreadcrumbSeparator className="hidden md:block" />
                    <BreadcrumbItem>
                      <BreadcrumbPage>{selected}</BreadcrumbPage>
                    </BreadcrumbItem>
                  </BreadcrumbList>
                </Breadcrumb>
              </div>
            </header>
            <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-4 pt-0">
              {Array.from({ length: 10 }).map((_, index) => (
                <div
                  key={index}
                  className="aspect-video max-w-3xl rounded-xl bg-muted/50"
                />
              ))}
            </div>
          </main>
        </SidebarProvider>
      </DialogContent>
    </Dialog>
  )
}
