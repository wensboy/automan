import * as React from "react"
import { Check, ChevronsUpDown, GalleryVerticalEnd } from "lucide-react"

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/**
 * 文档版本切换 (复刻 sidebar-02)。
 * 注意: DropdownMenuTrigger 嵌在 SidebarMenuButton(asChild) 之内 —— React 18 下反向嵌套会丢 ref,
 * 导致 Radix 拿不到锚点、菜单不可见 (见 v14 说明)。
 */
export default function VersionSwitcher({
  versions,
  defaultVersion,
}: {
  versions: string[]
  defaultVersion: string
}) {
  const [selectedVersion, setSelectedVersion] = React.useState(defaultVersion)

  // defaultVersion 来自异步加载的 meta(本地 outline): 挂载时可能还是空串,
  // 到达后需要跟随, 否则左上永远显示空版本(默认选中本地的语义就丢了)。
  React.useEffect(() => {
    if (defaultVersion) setSelectedVersion(defaultVersion)
  }, [defaultVersion])

  // meta.version 自带 "v" 前缀(如 v0.1.0-outline), 展示时统一只加一次
  const formatVersion = (version: string) => {
    if (!version) return "—"
    return /^v/i.test(version) ? version : `v${version}`
  }

  return (
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
              <div className="flex flex-col gap-0.5 leading-none">
                <span className="font-medium">{staticValue("sidebar.help.version")}</span>
                <span className="">{formatVersion(selectedVersion)}</span>
              </div>
              <ChevronsUpDown className="ml-auto" />
            </DropdownMenuTrigger>
          </SidebarMenuButton>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width)"
            align="start"
          >
            {versions.map((version) => (
              <DropdownMenuItem
                key={version}
                onSelect={() => setSelectedVersion(version)}
              >
                {formatVersion(version)}{" "}
                {version === selectedVersion && <Check className="ml-auto" />}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
