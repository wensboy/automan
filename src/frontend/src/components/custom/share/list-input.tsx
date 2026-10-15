import * as React from "react"
import { XIcon } from "lucide-react"
import { cn } from "cn"

/**
 * 列表输入 (list-input): 在同一个输入框内显示已固定的项。
 * - Enter        : 把当前文本固定为一项 (去空白、去重); 项以边框凸显
 * - hover 项上的 ×: 移除该项
 * - Backspace    : 输入框为空时, 完整回退最后一项
 */
export default function ListInput({
  value,
  onChange,
  placeholder,
  className,
  ...props
}: Omit<React.ComponentProps<"input">, "value" | "onChange"> & {
  value: string[]
  onChange: (value: string[]) => void
}) {
  const [draft, setDraft] = React.useState("")

  const pin = () => {
    const text = draft.trim()
    if (text === "") return
    if (!value.includes(text)) onChange([...value, text])
    setDraft("")
  }

  const removeAt = (index: number) => onChange(value.filter((_, i) => i !== index))

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") {
      event.preventDefault()
      pin()
      return
    }
    if (event.key === "Backspace" && draft === "" && value.length > 0) {
      event.preventDefault()
      removeAt(value.length - 1)
    }
  }

  return (
    <div
      className={cn(
        "border-input focus-within:border-ring focus-within:ring-ring/50 flex min-h-9 w-full flex-wrap items-center gap-1 rounded-md border bg-transparent px-2 py-1.5 text-sm shadow-xs transition-shadow focus-within:ring-[3px]",
        className,
      )}
    >
      {value.map((item, index) => (
        <span
          key={`${item}-${index}`}
          className="group flex items-center gap-1 rounded-sm border px-1.5 py-0.5 text-xs"
        >
          {item}
          <button
            type="button"
            aria-label={`Remove ${item}`}
            onClick={() => removeAt(index)}
            className="hover:text-destructive rounded-sm p-0.5 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
          >
            <XIcon className="size-3" />
          </button>
        </span>
      ))}
      <input
        {...props}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={value.length === 0 ? placeholder : undefined}
        className="placeholder:text-muted-foreground min-w-24 flex-1 bg-transparent outline-none"
      />
    </div>
  )
}
