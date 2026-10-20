import * as React from "react"
import { toast } from "sonner"

import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { staticValue } from "@/components/custom/share/provider/static-provider"
import { List as ListCommands, Run as RunCommand } from "../../../../bindings/github.com/wensboy/automan/backend/api/web/commandservice"
import type { Group } from "../../../../bindings/github.com/wensboy/automan/backend/service/command/models"

/**
 * 命令面板 (nav-secondary 的 Search 入口)。
 *
 * 数据来源只有一处: command 服务的 List()(= data/conf/command.json 的 runtime 段)。
 * 选中后调 Run(id) 让后端执行, 返回的 action 交给上层做 UI 变化(打开设置/登出…):
 * 前端不判断"这条命令是什么", 只按 action 反应 —— 与 VSCode 的命令模型一致。
 */
export default function CommandMenu({
  open,
  onOpenChange,
  onAction,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAction: (action: string, target?: string) => void
}) {
  const [groups, setGroups] = React.useState<Group[]>([])

  React.useEffect(() => {
    if (!open) return
    let cancelled = false
    void (async () => {
      try {
        const loaded = await ListCommands()
        if (!cancelled) setGroups(loaded ?? [])
      } catch (error) {
        toast.error(error instanceof Error ? error.message : String(error))
      }
    })()
    return () => {
      cancelled = true
    }
  }, [open])

  const run = async (id: string) => {
    onOpenChange(false)
    try {
      const result = await RunCommand(id)
      if (result?.action) onAction(result.action, result.target ?? undefined)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error))
    }
  }

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} showCloseButton={false}>
      <CommandInput placeholder={staticValue("sidebar.command.placeholder")} />
      <CommandList>
        <CommandEmpty>{staticValue("sidebar.command.empty")}</CommandEmpty>
        {groups
          .filter((group) => (group.items ?? []).length > 0)
          .map((group) => (
            <CommandGroup key={group.name} heading={group.name}>
              {(group.items ?? []).map((item) => (
                <CommandItem
                  key={item.id}
                  value={`${item.label} ${item.id}`}
                  onSelect={() => void run(item.id)}
                >
                  <span>{item.label}</span>
                  {item.kbd ? (
                    <kbd className="ml-auto rounded border bg-muted px-1 text-[10px] text-muted-foreground">
                      {item.kbd}
                    </kbd>
                  ) : null}
                </CommandItem>
              ))}
            </CommandGroup>
          ))}
      </CommandList>
    </CommandDialog>
  )
}
