import * as React from "react"
import { marked } from "marked"
import { EyeIcon } from "lucide-react"
import { cn } from "cn"

import SwitchButton from "@/components/custom/share/switch-button"

/**
 * Markdown 块: header(标题 + 渲染开关) + body(渲染结果 / 原文)。
 *
 * 约定:
 *   · header 从右到左排布: 最右是 switch btn(切换 markdown 是否渲染), 其左是标题;
 *   · body 自身滚动(y 溢出), 不把高度撑开(配合父级 min-h-0);
 *   · 当前不做 HTML 消毒(按需求: 安全 DOM 后续引入), 渲染结果直接注入。
 */
export default function MarkChunk({
  title,
  value,
  className,
  defaultRendered = true,
}: {
  title?: string
  /** markdown 原文 */
  value: string
  className?: string
  defaultRendered?: boolean
}) {
  const [rendered, setRendered] = React.useState(defaultRendered)

  const html = React.useMemo(() => {
    try {
      return marked.parse(value ?? "", { async: false }) as string
    } catch {
      return ""
    }
  }, [value])

  return (
    <div className={cn("flex min-h-0 flex-col overflow-hidden rounded-md border", className)}>
      <div className="flex h-8 items-center justify-between gap-2 border-b bg-muted/50 px-2">
        {/* title 可选: docs 场景不需要标题, 只留右侧开关 */}
        {title ? <span className="truncate text-xs text-muted-foreground">{title}</span> : <span />}
        <SwitchButton
          // 与 h-8 的 header 对齐: Button 的 icon 尺寸默认 size-9, 会溢出头部
          className="size-6 shrink-0"
          active={rendered}
          onToggle={() => setRendered((prev) => !prev)}
          aria-label={rendered ? "Show markdown source" : "Render markdown"}
          title={rendered ? "Show markdown source" : "Render markdown"}
        >
          <EyeIcon className="size-3.5" />
        </SwitchButton>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {rendered ? (
          <div
            className={cn(
              "max-w-none p-4 text-sm leading-relaxed",
              // 无 typography 插件: 用任意变体给常见 markdown 元素最小样式
              "[&_h1]:mb-3 [&_h1]:text-xl [&_h1]:font-semibold",
              "[&_h2]:mb-2 [&_h2]:mt-4 [&_h2]:text-lg [&_h2]:font-semibold",
              "[&_h3]:mb-2 [&_h3]:mt-3 [&_h3]:text-base [&_h3]:font-medium",
              "[&_p]:my-2 [&_ul]:my-2 [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:my-2 [&_ol]:list-decimal [&_ol]:pl-5",
              "[&_li]:my-1 [&_a]:text-primary [&_a]:underline",
              "[&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-xs",
              "[&_pre]:my-3 [&_pre]:overflow-x-auto [&_pre]:rounded-md [&_pre]:bg-muted [&_pre]:p-3",
              "[&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground",
              "[&_table]:my-3 [&_table]:w-full [&_th]:border [&_th]:px-2 [&_th]:py-1 [&_td]:border [&_td]:px-2 [&_td]:py-1",
              "[&_hr]:my-4 [&_img]:max-w-full",
            )}
            dangerouslySetInnerHTML={{ __html: html }}
          />
        ) : (
          <pre className="whitespace-pre-wrap break-words p-4 font-mono text-xs leading-relaxed">
            {value}
          </pre>
        )}
      </div>
    </div>
  )
}
