import type { ReactNode } from "react"
import { PencilIcon, RefreshCwIcon, SaveIcon } from "lucide-react"
import { cn } from "cn"

import SwitchButton from "@/components/custom/share/switch-button"
import { Button } from "@/components/ui/button"

/**
 * 带标签的块容器 (与 LabelInput 同族, 但内容不限组件)。
 * - v 方向: 上 header / 下 body, 中间以边框黏连分隔; header 左为 label,
 *   右为图标按钮组 (DOM 顺序 [刷新][保存][编辑开关] → 视觉最右是编辑)。
 * - h 方向: 左 label / 右 body, 中间以竖边框分隔 (动作按钮若有则贴最右)。
 * 内容区不做限制; 传入可编辑内容时, 用 editing 驱动其只读状态 (由 header 右侧的编辑开关切换)。
 */
export default function LabelChunk({
  label,
  direction = "v",
  className,
  editing = false,
  onToggleEdit,
  onSave,
  onRefresh,
  children,
}: {
  label: string
  direction?: "v" | "h"
  className?: string
  editing?: boolean
  onToggleEdit?: () => void
  onSave?: () => void
  onRefresh?: () => void
  children: ReactNode
}) {
  const hasActions = Boolean(onToggleEdit || onSave || onRefresh)
  const actions = hasActions ? (
    <div className="flex items-center gap-1">
      {onRefresh && (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-6"
          aria-label="Refresh"
          onClick={onRefresh}
        >
          <RefreshCwIcon className="size-3.5" />
        </Button>
      )}
      {onSave && (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-6"
          aria-label="Save"
          onClick={onSave}
        >
          <SaveIcon className="size-3.5" />
        </Button>
      )}
      {onToggleEdit && (
        <SwitchButton
          active={editing}
          onToggle={onToggleEdit}
          className="size-6"
          aria-label="Edit"
        >
          <PencilIcon className="size-3.5" />
        </SwitchButton>
      )}
    </div>
  ) : null

  if (direction === "h") {
    return (
      <div
        className={cn("flex items-stretch overflow-hidden rounded-md border", className)}
      >
        <span className="flex items-center border-r bg-muted/50 px-3 text-sm text-muted-foreground">
          {label}
        </span>
        <div className="flex min-w-0 flex-1 items-center">{children}</div>
        {actions && <div className="flex items-center border-l px-1">{actions}</div>}
      </div>
    )
  }

  return (
    <div className={cn("flex flex-col overflow-hidden rounded-md border", className)}>
      <div className="flex items-center justify-between gap-2 border-b bg-muted/50 px-3 py-1">
        <span className="text-sm text-muted-foreground">{label}</span>
        {actions}
      </div>
      <div className="flex min-h-0 flex-1 flex-col">{children}</div>
    </div>
  )
}
