import * as React from "react"
import { XIcon } from "lucide-react"
import { cn } from "cn"
import { staticValue } from "@/components/custom/share/provider/static-provider"

export type TabItem = {
  value: string
  label: string
}

/**
 * 标签栏 (VSCode 风格): 横向排列, 单个标签可关闭 (悬停/聚焦时显示), 溢出时用鼠标滚轮横向滚动。
 * 纯展示: 当前选中项与关闭动作都由调用方持有 —— 这里只负责渲染与滚动。
 */
export default function TabBar({
  items,
  value,
  onValueChange,
  onClose,
  className,
}: {
  items: TabItem[]
  value: string
  onValueChange: (value: string) => void
  onClose?: (value: string) => void
  className?: string
}) {
  const stripRef = React.useRef<HTMLDivElement | null>(null)

  // 纵向滚轮转横向滚动 (仅在该标签栏确实溢出时才动, 否则不影响外层)
  const handleWheel = (event: React.WheelEvent<HTMLDivElement>) => {
    const strip = stripRef.current
    if (!strip || strip.scrollWidth <= strip.clientWidth) return
    strip.scrollLeft += event.deltaY !== 0 ? event.deltaY : event.deltaX
  }

  return (
    <div
      ref={stripRef}
      onWheel={handleWheel}
      role="tablist"
      className={cn(
        // 滚动条已由 index.css 全局隐藏(零宽不占位); 溢出靠鼠标滚轮横向滚动 (见 handleWheel)
        "flex min-w-0 items-stretch gap-1 overflow-x-auto",
        className,
      )}
    >
      {items.map((item) => {
        const active = item.value === value
        return (
          <div
            key={item.value}
            className={cn(
              // 标签之间不用分隔线, 靠父层 gap 区分; 圆角与 ui/tabs 的 trigger 保持一致(rounded-md)
              "group flex h-full shrink-0 items-center gap-1 rounded-md pr-1 pl-2 text-sm",
              // 同色调凸显: 选中项只比背景深/浅一档 (浅色更灰、深色更亮), 不做反色;
              // 未选中与所在背景一致, 仅在悬停时给一点反馈
              active
                ? "bg-muted text-foreground"
                : "bg-transparent text-muted-foreground hover:bg-muted/40",
            )}
          >
            <button
              type="button"
              role="tab"
              aria-selected={active}
              onClick={() => onValueChange(item.value)}
              className="max-w-40 truncate text-left"
            >
              {item.label}
            </button>
            {onClose && (
              <button
                type="button"
                aria-label={`${staticValue("contentArea.home.closeTab") ?? ""}${item.label}`}
                onClick={() => onClose(item.value)}
                // 悬停时图标转红: 只用文字色反馈, 这样在选中(primary 实心)与未选中标签上都成立
                className="rounded-sm p-0.5 opacity-0 group-hover:opacity-100 focus-visible:opacity-100 hover:text-destructive"
              >
                <XIcon className="size-3.5" />
              </button>
            )}
          </div>
        )
      })}
    </div>
  )
}
