import { SearchIcon } from "lucide-react"

import RichInput from "@/components/custom/share/rich-input"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/** 通知搜索: 图标前缀 + 输入框 (复用 share/rich-input, 保持调用方接口稳定)。 */
export default function NoticeSearch({
  value,
  onChange,
  placeholder = "Search",
}: {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}) {
  return (
    <RichInput
      prefix={<SearchIcon />}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      placeholder={placeholder ?? staticValue("sidebar.notice.searchPlaceholder")}
      aria-label={staticValue("sidebar.notice.searchAria")}
    />
  )
}
