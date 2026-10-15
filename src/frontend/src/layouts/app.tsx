import { Fragment } from "react"
import { Outlet, useMatches } from "react-router"

import AppSidebar from "@/components/custom/sidebar/app-sidebar"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { Separator } from "@/components/ui/separator"
import { SidebarInset, SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar"

type RouteHandle = { crumb?: string }

/** 应用布局: 侧边栏 + 内容区骨架; 面包屑取自当前路由链的 handle.crumb。 */
export default function AppLayout() {
  const crumbs = useMatches()
    .map((match) => (match.handle as RouteHandle | undefined)?.crumb)
    .filter((crumb): crumb is string => Boolean(crumb))

  return (
    <SidebarProvider>
      <AppSidebar />
      {/*
        外壳固定为视口高度并裁掉溢出: 内容区不再随内容增长 (否则长内容会把 body 撑出滚动条),
        需要滚动的页面在自己的内容区内部滚动 (见 v14 "外壳高度与内部滚动")。
      */}
      <SidebarInset className="h-svh overflow-hidden">
        <header className="flex h-16 shrink-0 items-center gap-2 transition-[width,height] ease-linear group-has-[[data-collapsible=icon]]/sidebar-wrapper:h-12">
          <div className="flex items-center gap-2 px-4">
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-2 h-4" />
            <Breadcrumb>
              <BreadcrumbList>
                {crumbs.map((crumb, index) => (
                  <Fragment key={crumb}>
                    {index > 0 && <BreadcrumbSeparator className="hidden md:block" />}
                    <BreadcrumbItem className={index < crumbs.length - 1 ? "hidden md:block" : undefined}>
                      {index < crumbs.length - 1 ? (
                        <BreadcrumbLink href="#">{crumb}</BreadcrumbLink>
                      ) : (
                        <BreadcrumbPage>{crumb}</BreadcrumbPage>
                      )}
                    </BreadcrumbItem>
                  </Fragment>
                ))}
              </BreadcrumbList>
            </Breadcrumb>
          </div>
        </header>
        <Outlet />
      </SidebarInset>
    </SidebarProvider>
  )
}
