import * as React from "react"
import { BellOffIcon, Trash2Icon } from "lucide-react"

import SwitchButton from "@/components/custom/share/switch-button"
import { Button } from "@/components/ui/button"
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer"
import { ScrollArea } from "@/components/ui/scroll-area"
import { notices as seedNotices, type Notice } from "@/mock/notifications"

import NoticeItem from "./notice-item"
import NoticeSearch from "./notice-search"
import { staticValue } from "@/components/custom/share/provider/static-provider"

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
  const [items, setItems] = React.useState<Notice[]>(seedNotices)
  const [query, setQuery] = React.useState("")
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
              onClick={() => setItems([])}
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
                  onDismiss={(id) =>
                    setItems((prev) => prev.filter((item) => item.id !== id))
                  }
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
