import * as React from "react"
import { cn } from "cn"

import { Input } from "@/components/ui/input"

/**
 * 带固定前缀标签的输入框: label 与 input 黏连, 之间用右边框分隔;
 * 前缀是固定字符串 (字段含义), 这是它与 RichInput 的唯一区别。
 * readOnly 时整块降一档色调, 与可编辑态区分。
 */
export default function LabelInput({
  label,
  className,
  readOnly,
  ...props
}: React.ComponentProps<typeof Input> & {
  label: string
}) {
  return (
    <div
      className={cn(
        "flex items-stretch overflow-hidden rounded-md border",
        readOnly
          ? "bg-muted/40"
          : "focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50",
      )}
    >
      <span className="flex items-center border-r bg-muted/50 px-3 text-sm text-muted-foreground">
        {label}
      </span>
      <Input
        readOnly={readOnly}
        className={cn(
          "rounded-none border-0 bg-transparent shadow-none focus-visible:ring-0 dark:bg-transparent",
          className,
        )}
        {...props}
      />
    </div>
  )
}
