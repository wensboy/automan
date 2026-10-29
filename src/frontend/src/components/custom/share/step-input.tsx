import { staticValue } from "@/components/custom/share/provider/static-provider"
import * as React from "react"
import { ChevronDownIcon, ChevronUpIcon } from "lucide-react"
import { cn } from "cn"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

/**
 * 步进输入: 左 input(可手动编辑) + 右上下步进按钮(上 +1 / 下 -1)。
 *
 * 取舍:
 *   · 用 shadcn 的 Input(type=text + inputMode=numeric), **不用原生 number** —— 去掉原生上下箭头,
 *     避免滚轮误改与浏览器差异;
 *   · 输入过程中保留文本(允许 "6" -> "60" 这类中间态), 在 blur / Enter 时解析提交;
 *     非法输入回退到当前生效值; 有 min/max 时夹紧;
 *   · 步进按钮直接提交, 同时把 input 文本同步为最新值。
 */
export default function StepInput({
  value,
  onChange,
  min,
  max,
  step = 1,
  id,
  disabled = false,
  className,
}: {
  value: number
  onChange: (next: number) => void
  min?: number
  max?: number
  step?: number
  id?: string
  disabled?: boolean
  className?: string
}) {
  const finite = Number.isFinite(value)
  const current = finite ? value : 0

  const [text, setText] = React.useState(() => (finite ? String(value) : ""))

  // 外部值变化(比如其它控件联动)时同步文本
  React.useEffect(() => {
    setText(Number.isFinite(value) ? String(value) : "")
  }, [value])

  const clamp = (next: number) =>
    Math.min(max ?? Number.POSITIVE_INFINITY, Math.max(min ?? Number.NEGATIVE_INFINITY, next))

  const commit = (raw: string) => {
    const parsed = Number.parseFloat(raw.trim())
    if (!Number.isFinite(parsed)) {
      setText(finite ? String(value) : "")
      return
    }
    const next = clamp(parsed)
    setText(String(next))
    if (next !== value) onChange(next)
  }

  const bump = (delta: number) => {
    if (disabled) return
    const next = clamp(current + delta)
    setText(String(next))
    if (next !== current) onChange(next)
  }

  const canIncrease = !disabled && (max === undefined ? true : current < max)
  const canDecrease = !disabled && (min === undefined ? true : current > min)

  return (
    <div
      className={cn(
        "flex h-8 w-full items-stretch overflow-hidden rounded-md border focus-within:border-ring",
        disabled && "opacity-50",
        className,
      )}
    >
      <Input
        id={id}
        value={text}
        disabled={disabled}
        inputMode="numeric"
        autoComplete="off"
        className="h-full min-w-0 flex-1 rounded-none border-0 shadow-none focus-visible:ring-0"
        onChange={(event) => setText(event.target.value)}
        onBlur={() => commit(text)}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            commit(text)
            event.currentTarget.blur()
          }
          if (event.key === "Escape") setText(finite ? String(value) : "")
        }}
      />
      {/* 右侧上下步进: 上 +1 / 下 -1(贴合 shadcn 的紧凑图标按钮, 不使用原生箭头) */}
      <div className="flex shrink-0 flex-col border-l">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-1/2 w-6 rounded-none"
          disabled={!canIncrease}
          onClick={() => bump(step)}
          aria-label={staticValue("contentArea.shared.actions.increase")}
        >
          <ChevronUpIcon className="size-3" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-1/2 w-6 rounded-none border-t"
          disabled={!canDecrease}
          onClick={() => bump(-step)}
          aria-label={staticValue("contentArea.shared.actions.decrease")}
        >
          <ChevronDownIcon className="size-3" />
        </Button>
      </div>
    </div>
  )
}
