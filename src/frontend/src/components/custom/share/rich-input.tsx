import * as React from "react"
import { cn } from "cn"

import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"

/**
 * 富输入框: Input (或 multiline 时的 Textarea) + 前后缀插槽 (图标前缀 / 快捷键提示后缀等)。
 * 不传插槽时行为与原控件完全一致, 因此可直接替代 Input/Textarea 使用。
 * 前缀默认不接收指针事件(纯装饰), 后缀保持可交互(可放按钮/清除等)。
 */
export default function RichInput({
  prefix,
  suffix,
  className,
  multiline = false,
  ...props
}: Omit<React.ComponentProps<typeof Input>, "prefix" | "suffix"> &
  Partial<Omit<React.ComponentProps<typeof Textarea>, "prefix" | "suffix">> & {
    prefix?: React.ReactNode
    suffix?: React.ReactNode
    multiline?: boolean
  }) {
  // 多行时插槽贴顶对齐 (单行居中), 否则前缀会落到整块中间。
  const slotClass = multiline ? "top-2" : "top-1/2 -translate-y-1/2"
  const controlClass = cn(prefix && "pl-8", suffix && "pr-8", className)

  return (
    <div className="relative">
      {prefix && (
        <span
          className={cn(
            "pointer-events-none absolute left-2.5 flex items-center text-muted-foreground [&_svg]:size-4",
            slotClass,
          )}
        >
          {prefix}
        </span>
      )}
      {multiline ? (
        <Textarea
          className={controlClass}
          {...(props as React.ComponentProps<typeof Textarea>)}
        />
      ) : (
        <Input className={controlClass} {...props} />
      )}
      {suffix && (
        <span
          className={cn(
            "absolute right-2.5 flex items-center text-muted-foreground [&_svg]:size-4",
            slotClass,
          )}
        >
          {suffix}
        </span>
      )}
    </div>
  )
}
