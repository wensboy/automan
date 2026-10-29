import { Search } from "lucide-react"

import { Label } from "@/components/ui/label"
import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarInput,
} from "@/components/ui/sidebar"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/** 侧栏内的文档搜索框 (复刻 sidebar-02; dev 阶段仅为静态输入框)。 */
export default function SearchForm({ ...props }: React.ComponentProps<"form">) {
  return (
    <form {...props}>
      <SidebarGroup className="py-0">
        <SidebarGroupContent className="relative">
          <Label htmlFor="search" className="sr-only">
            {staticValue("contentArea.help.search")}
          </Label>
          <SidebarInput
            id="search"
            placeholder={staticValue("sidebar.help.searchPlaceholder")}
            className="pl-8"
          />
          <Search className="pointer-events-none absolute top-1/2 left-2 size-4 -translate-y-1/2 opacity-50 select-none" />
        </SidebarGroupContent>
      </SidebarGroup>
    </form>
  )
}
