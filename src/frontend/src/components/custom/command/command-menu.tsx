import * as React from "react"
import { toast } from "sonner"

import { settingsGroups } from "@/lib/settings"

import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { useStatic, staticValue } from "@/components/custom/share/provider/static-provider"



/**
 * 命令面板 (由 nav-secondary 的 Search 项触发)。
 * 不自建弹层: 用 shadcn 的 CommandDialog (Dialog + Command 的官方组合, 标题/描述已 sr-only 处理)。
 * 数据复用既有来源 —— 侧栏导航取自 static, 设置分区取自 mock; dev 阶段选中后只给可见反馈。
 */
export default function CommandMenu({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { icon, sidebar } = useStatic()
  // 设置分区: 结构来自 static/default/settings.json, 展示文案/图标来自 sidebar.json;
  // 图标名必须解析成组件再渲染 —— 直接写 <section.icon /> 会把 "video"/"link" 当成 HTML 标签
  const sections = React.useMemo(
    () => settingsGroups().map((group) => ({ ...group, Icon: icon(group.icon) })),
    [icon],
  )

  const run = (label: string) => {
    onOpenChange(false)
    // dev 阶段: 真实动作 (路由跳转 / 打开对应弹窗) 待接, 先给反馈
    toast.success(`${staticValue("sidebar.command.selected") ?? ""}${label}`)
  }

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} showCloseButton={false}>
      <CommandInput placeholder={staticValue("sidebar.command.placeholder")} />
      <CommandList>
        <CommandEmpty>{staticValue("sidebar.command.empty")}</CommandEmpty>
        <CommandGroup heading={staticValue("sidebar.command.pagesGroup")}>
          {sidebar.navMain.map((item) => (
            <CommandItem
              key={item.title}
              value={item.title}
              onSelect={() => run(item.title)}
            >
              {item.icon && <item.icon />}
              <span>{item.title}</span>
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandGroup heading={staticValue("sidebar.command.settingsGroup")}>
          {sections.map((section) => (
            <CommandItem
              key={section.id}
              value={section.label}
              onSelect={() => run(section.label)}
            >
              <section.Icon />
              <span>{section.label}</span>
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
