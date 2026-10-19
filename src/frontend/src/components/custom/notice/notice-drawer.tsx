import * as React from "react"
import { BellOffIcon, Trash2Icon } from "lucide-react"

import SwitchButton from "@/components/custom/share/switch-button"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer"
import { ScrollArea } from "@/components/ui/scroll-area"

import NoticeItem from "./notice-item"
import NoticeSearch from "./notice-search"
import { staticValue } from "@/components/custom/share/provider/static-provider"
import {
  clearNotices,
  dismissNotice,
  markNoticesRead,
  snapshotNoticeState,
  subscribeNotices,
} from "@/lib/notice"

/**
 * 通知抽屉: 从右侧滑出, 自上而下为 标题 / 搜索 / 操作行 / 可滚动列表。
 * 受控组件 (open/onOpenChange); 数据来自 mock, 列表的增删只作用于本层副本。
 */
export default function NoticeDrawer({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  // 数据源是后端队列: 事件到达即刷新(useSyncExternalStore 避免手写订阅)
  const { items } = React.useSyncExternalStore(subscribeNotices, snapshotNoticeState)
  const [query, setQuery] = React.useState("")

  // 打开抽屉 = 已读(铃铛红点归零); 队列本身不清
  React.useEffect(() => {
    if (open) markNoticesRead()
  }, [open])
  const [dnd, setDnd] = React.useState(false)

  const keyword = query.trim().toLowerCase()
  const visible =
    keyword === ""
      ? items
      : items.filter((notice) =>
          [notice.label, notice.title, notice.content].some((field) =>
            field.toLowerCase().includes(keyword),
          ),
        )

  return (
    <Drawer open={open} onOpenChange={onOpenChange} direction="right">
      <DrawerContent aria-describedby={undefined}>
        <DrawerHeader className="text-center">
          <DrawerTitle>{staticValue("sidebar.notice.title")}</DrawerTitle>
        </DrawerHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-4 p-4 pt-0">
          <NoticeSearch value={query} onChange={setQuery} />
          <div className="flex items-center justify-end gap-2">
            <SwitchButton
              active={dnd}
              onToggle={() => setDnd((prev) => !prev)}
              aria-label={staticValue("sidebar.notice.doNotDisturb")}
            >
              <BellOffIcon className="size-4" />
            </SwitchButton>
            <Button
              type="button"
              variant="outline"
              size="icon"
              aria-label={staticValue("sidebar.notice.clear")}
              onClick={() => {
                // 先清后端队列(保证重启/其它窗口一致), 失败则提示并保留列表
                void clearNotices().catch((error: unknown) => {
                  toast.error(error instanceof Error ? error.message : String(error))
                })
              }}
            >
              <Trash2Icon className="size-4" />
            </Button>
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <div className="flex flex-col gap-3 pr-3">
              {visible.map((notice) => (
                <NoticeItem
                  key={notice.id}
                  notice={notice}
                  onDismiss={dismissNotice}
                />
              ))}
              {visible.length === 0 && (
                <p className="py-6 text-center text-sm text-muted-foreground">
                  {staticValue("sidebar.notice.empty")}
                </p>
              )}
            </div>
          </ScrollArea>
        </div>
      </DrawerContent>
    </Drawer>
  )
}
