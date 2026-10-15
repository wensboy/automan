import * as React from "react"
import { type LucideIcon } from "lucide-react"

import CommandMenu from "@/components/custom/command/command-menu"
import HelpDialog from "@/components/custom/help/help-dialog"
import SettingsDialog from "@/components/custom/settings/settings-dialog"
import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"

/** 次要导航: 贴底展示; 每一项都是一个入口 (弹窗 / 命令面板), 主体不在侧栏内。 */
export default function NavSecondary({
  items,
  ...props
}: {
  items: {
    title: string
    url: string
    icon: LucideIcon
  }[]
} & React.ComponentProps<typeof SidebarGroup>) {
  const [openTitle, setOpenTitle] = React.useState<string | null>(null)

  return (
    <>
      <SidebarGroup {...props}>
        <SidebarGroupContent>
          <SidebarMenu>
            {items.map((item) => (
              <SidebarMenuItem key={item.title}>
                <SidebarMenuButton asChild>
                  <button type="button" onClick={() => setOpenTitle(item.title)}>
                    <item.icon />
                    <span>{item.title}</span>
                  </button>
                </SidebarMenuButton>
              </SidebarMenuItem>
            ))}
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>
      <SettingsDialog
        open={openTitle === "Settings"}
        onOpenChange={(open) => !open && setOpenTitle(null)}
      />
      <HelpDialog
        open={openTitle === "Help"}
        onOpenChange={(open) => !open && setOpenTitle(null)}
      />
      <CommandMenu
        open={openTitle === "Search"}
        onOpenChange={(open) => !open && setOpenTitle(null)}
      />
    </>
  )
}
