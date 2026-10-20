import * as React from "react"

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarRail,
} from "@/components/ui/sidebar"

import NavMain from "./nav-main"
import NavSecondary from "./nav-secondary"
import NavUser from "./nav-user"
import WorkspaceSwitcher from "./workspace-switcher"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/** 应用侧边栏: 结构固定, 数据来自静态数据 provider。 */
export default function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const { sidebar } = useStatic()

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <WorkspaceSwitcher />
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={sidebar.navMain} />
        <NavSecondary items={sidebar.navSecondary} className="mt-auto" />
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={sidebar.user} />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
