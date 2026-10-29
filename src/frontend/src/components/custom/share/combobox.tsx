import { staticValue } from "@/components/custom/share/provider/static-provider"
import * as React from "react"
import { CheckIcon, ChevronDownIcon, XIcon } from "lucide-react"

import { Input } from "@/components/ui/input"
import { Popover, PopoverAnchor, PopoverContent } from "@/components/ui/popover"
import { cn } from "cn"

/**
 * 轻量 Combobox(见 docs/design/v25.md §19.8)。
 *
 * 为什么自行实现: shadcn 新版 Combobox 基于 Base UI, 在本项目的"内联输入 + 弹层"
 * 组合下出现首次输入失焦/宽度异常等问题; 这里用 Radix Popover + 纯列表实现,
 * 行为可控且与现有 Radix 组件栈一致:
 *   · 输入框始终是焦点所在(弹层不抢占焦点: autofocus 一律 preventDefault);
 *   · 弹层宽度 = 输入框实测宽度, 固定向上弹出;
 *   · 有值时尾部图标切换为清除按钮, 清除后输入与选中同时清空。
 */

export type ComboboxItem = { value: string; label: string }

/** 单行展示: 超长时中间用 … 省略(保留头尾, 适合路径)。 */
function middleEllipsis(text: string, max = 46): string {
  if (text.length <= max) return text
  const head = Math.ceil((max - 1) / 2)
  const tail = Math.floor((max - 1) / 2)
  return `${text.slice(0, head)}…${text.slice(-tail)}`
}

type ComboboxProps = {
  items: ComboboxItem[]
  value?: string
  placeholder?: string
  emptyText?: string
  onChange: (value: string | undefined) => void
  className?: string
}

export default function Combobox({
  items,
  value,
  placeholder,
  emptyText = "No results.",
  onChange,
  className,
}: ComboboxProps) {
  const inputRef = React.useRef<HTMLInputElement | null>(null)
  const anchorRef = React.useRef<HTMLDivElement | null>(null)
  const [open, setOpen] = React.useState(false)
  const [query, setQuery] = React.useState("")
  const [width, setWidth] = React.useState<number | undefined>()
  const [active, setActive] = React.useState(0)

  const selected = items.find((item) => item.value === value)
  const display = selected ? selected.label : query

  const filtered = React.useMemo(() => {
    const keyword = query.trim().toLowerCase()
    if (!keyword) return items
    return items.filter((item) => item.label.toLowerCase().includes(keyword))
  }, [items, query])

  React.useEffect(() => setActive(0), [query, open])

  const measure = () => setWidth(anchorRef.current?.offsetWidth)

  const commit = (item: ComboboxItem) => {
    onChange(item.value)
    setQuery("")
    setOpen(false)
    inputRef.current?.focus()
  }

  const clear = () => {
    onChange(undefined)
    setQuery("")
    setOpen(false)
    inputRef.current?.focus()
  }

  return (
    <Popover open={open} onOpenChange={(next) => (next ? (measure(), setOpen(true)) : setOpen(false))}>
      <PopoverAnchor asChild>
        <div ref={anchorRef} className={cn("relative w-full", className)}>
          <Input
            ref={inputRef}
            value={display}
            placeholder={placeholder}
            className="h-7 pr-7 text-xs"
            onFocus={() => {
              measure()
              setOpen(true)
            }}
            onClick={() => {
              measure()
              setOpen(true)
            }}
            onChange={(event) => {
              setQuery(event.target.value)
              if (value !== undefined) onChange(undefined)
              measure()
              setOpen(true)
            }}
            onKeyDown={(event) => {
              if (event.key === "ArrowDown") {
                event.preventDefault()
                setOpen(true)
                setActive((prev) => Math.min(prev + 1, Math.max(0, filtered.length - 1)))
              } else if (event.key === "ArrowUp") {
                event.preventDefault()
                setActive((prev) => Math.max(prev - 1, 0))
              } else if (event.key === "Enter") {
                if (!open) return
                event.preventDefault()
                const item = filtered[active]
                if (item) commit(item)
              } else if (event.key === "Escape") {
                setOpen(false)
              }
            }}
          />

          {selected || query ? (
            <button
              type="button"
              aria-label={staticValue("contentArea.shared.actions.clear")}
              className="absolute top-1/2 right-1 -translate-y-1/2 rounded-sm p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
              onMouseDown={(event) => event.preventDefault()}
              onClick={clear}
            >
              <XIcon className="size-3.5" />
            </button>
          ) : (
            <ChevronDownIcon className="pointer-events-none absolute top-1/2 right-2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          )}
        </div>
      </PopoverAnchor>

      <PopoverContent
        side="top"
        align="start"
        sideOffset={6}
        style={width ? { width, minWidth: width } : undefined}
        className="p-1"
        onOpenAutoFocus={(event) => event.preventDefault()}
        onCloseAutoFocus={(event) => event.preventDefault()}
        onMouseDown={(event) => event.preventDefault()}
      >
        <div className="max-h-56 overflow-y-auto">
          {filtered.length === 0 ? (
            <div className="px-2 py-2 text-xs text-muted-foreground">{emptyText}</div>
          ) : (
            filtered.map((item, index) => (
              <button
                key={item.value}
                type="button"
                className={cn(
                  "flex w-full items-center rounded-sm px-2 py-1.5 text-left text-xs",
                  index === active ? "bg-muted" : "hover:bg-muted/60",
                )}
                onMouseEnter={() => setActive(index)}
                onClick={() => commit(item)}
              >
                <span className={cn("min-w-0 flex-1 pr-2", index === active ? "whitespace-nowrap" : "truncate")}>
                  {index === active ? middleEllipsis(item.label) : item.label}
                </span>
                {item.value === value ? <CheckIcon className="ml-2 size-3.5 shrink-0" /> : null}
              </button>
            ))
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
