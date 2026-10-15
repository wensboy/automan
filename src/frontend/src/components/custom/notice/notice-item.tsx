import * as React from "react"
import { ChevronDownIcon, XIcon } from "lucide-react"
import { cn } from "cn"

import { Button } from "@/components/ui/button"
import type { Notice } from "@/mock/notifications"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/**
 * 单条通知: header 左侧 label, 右侧图标按钮 (展开 / 关闭);
 * body 为 title(单行截断) + content(2 行截断, 展开后完整显示)。
 */
export default function NoticeItem({
  notice,
  onDismiss,
}: {
  notice: Notice
  onDismiss?: (id: string) => void
}) {
  const [expanded, setExpanded] = React.useState(false)

  return (
    <div className="rounded-lg border p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-medium text-muted-foreground">{notice.label}</span>
        <div className="flex items-center gap-0.5">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-6"
            aria-label={expanded ? staticValue("sidebar.notice.collapse") : staticValue("sidebar.notice.expand")}
            aria-expanded={expanded}
            onClick={() => setExpanded((prev) => !prev)}
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
            onClick={() => onDismiss?.(notice.id)}
          >
            <XIcon className="size-3.5" />
          </Button>
        </div>
      </div>
      <div className="mt-2 space-y-1">
        <p className="truncate text-sm font-medium">{notice.title}</p>
        <p
          className={cn(
            "text-xs text-muted-foreground",
            expanded ? "whitespace-pre-wrap" : "line-clamp-2",
          )}
        >
          {notice.content}
        </p>
      </div>
    </div>
  )
}
