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
  loadNotices,
  markNoticesRead,
  snapshotNoticeState,
  subscribeNotices,
} from "@/lib/notice"
import { Get, Set } from "../../../../bindings/github.com/wensboy/automan/backend/api/web/settingsservice"

/** 勿扰开关对应的设置键(服务端每次 Push 都会重新读它)。 */
const DND_KEY = "general.notice.doNotDisturb"

/**
 * 通知抽屉: 从右侧滑出, 自上而下为 标题 / 搜索 / 操作行 / 可滚动列表。
 * 受控组件 (open/onOpenChange); 数据来自后端队列(打开时同步), 未读/勿扰见 lib/notice 与设置项。
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

  // 打开抽屉: 先与后端队列同步(列表只展示"当前在 notification 里"的条目), 再标记已读。
  // 队列本身不清; 清除是显式操作(下面按钮)。
  React.useEffect(() => {
    if (!open) return
    void loadNotices()
    markNoticesRead()
  }, [open])

  // 勿扰是**设置项**(general.notice.doNotDisturb): 打开抽屉时读回真实值, 切换即落库;
  // 服务端每次 Push 都会重新读它, 因此切换立即生效(开启时既不弹 toast 也不入队)。
  const [dnd, setDnd] = React.useState(false)
  React.useEffect(() => {
    if (!open) return
    void Get(DND_KEY)
      .then((entry) => setDnd(Boolean(entry?.value)))
      .catch(() => setDnd(false))
  }, [open])

  const toggleDND = (): void => {
    const next = !dnd
    void Set(DND_KEY, next)
      .then(() => setDnd(next))
      .catch((error: unknown) => {
        toast.error(error instanceof Error ? error.message : String(error))
      })
  }

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
        <DrawerHeader className="flex flex-row items-center justify-between gap-2">
          <DrawerTitle>{staticValue("sidebar.notice.title")}</DrawerTitle>
          {/* 动作按钮收进 header(右侧): 勿扰 / 清除; 行为不变 */}
          <div className="flex items-center gap-1">
            <SwitchButton
              active={dnd}
              onToggle={toggleDND}
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
        </DrawerHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-4 p-4 pt-0">
          <NoticeSearch value={query} onChange={setQuery} />
          {/*
            ScrollArea 内部把内容包成 display:table, 单行文本(nowrap)会把宽度撑出视口;
            这里强制改回 block, 让条目的 truncate 生效。滑块是 Radix 自绘元素, 一并收起。
          */}
          <ScrollArea className="min-h-0 flex-1 [&_[data-slot=scroll-area-scrollbar]]:hidden [&_[data-slot=scroll-area-viewport]>div]:!block">
            <div className="flex flex-col gap-3">
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
