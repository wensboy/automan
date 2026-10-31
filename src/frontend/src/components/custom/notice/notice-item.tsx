import * as React from "react"
import { ChevronDownIcon, XIcon } from "lucide-react"
import { cn } from "cn"

import { Button } from "@/components/ui/button"
import type { Entry as Notice } from "../../../../bindings/github.com/wensboy/automan/backend/service/notice/models"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/**
 * 单条通知: header 左侧 label, 右侧图标按钮 (展开 / 关闭)。
 * body 为 title + content: 默认都是**单行截断**(不横向溢出), 点击卡片任意处(或展开按钮)展示全文。
 */
export default function NoticeItem({
  notice,
  onDismiss,
}: {
  notice: Notice
  onDismiss?: (id: string) => void
}) {
  const [expanded, setExpanded] = React.useState(false)
  const toggle = () => setExpanded((previous) => !previous)

  return (
    <div
      role="button"
      tabIndex={0}
      aria-expanded={expanded}
      onClick={toggle}
      onKeyDown={(event) => {
        if (event.key !== "Enter" && event.key !== " ") return
        event.preventDefault()
        toggle()
      }}
      className="cursor-pointer rounded-lg border p-3 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
    >
      <div className="flex items-center justify-between gap-2">
        {/* label 是可选分类标记: 为空时整段不渲染, 避免头部留白(操作类信息不应进 notice, 见 v20 §10) */}
        {notice.label ? (
          <span className="truncate text-xs font-medium text-muted-foreground">{notice.label}</span>
        ) : null}
        <div className="ml-auto flex shrink-0 items-center gap-0.5">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-6"
            aria-label={expanded ? staticValue("sidebar.notice.collapse") : staticValue("sidebar.notice.expand")}
            aria-expanded={expanded}
            onClick={(event) => {
              event.stopPropagation()
              toggle()
            }}
          >
            <ChevronDownIcon
              className={cn("size-3.5 transition-transform", expanded && "rotate-180")}
            />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-6 hover:text-destructive"
            aria-label={staticValue("sidebar.notice.dismiss")}
            onClick={(event) => {
              event.stopPropagation()
              onDismiss?.(notice.id)
            }}
          >
            <XIcon className="size-3.5" />
          </Button>
        </div>
      </div>
      <div className="mt-2 space-y-1">
        {/* 标题是主信息: 默认单行截断, 展开后完整显示(长路径用 break-all 防止横向溢出) */}
        {notice.title ? (
          <p
            className={cn(
              "text-sm font-medium",
              expanded ? "break-words whitespace-pre-wrap" : "truncate",
            )}
          >
            {notice.title}
          </p>
        ) : null}
        {notice.content ? (
          <p
            className={cn(
              "text-xs text-muted-foreground",
              expanded ? "break-all whitespace-pre-wrap" : "truncate",
            )}
          >
            {notice.content}
          </p>
        ) : null}
        {/* 两者都空: 用 label 兜底, 保证卡片不是纯空白 */}
        {!notice.title && !notice.content ? (
          <p className="text-sm text-muted-foreground">{notice.label}</p>
        ) : null}
      </div>
    </div>
  )
}
