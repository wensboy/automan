import { RefreshCwIcon, SearchIcon, Trash2Icon } from "lucide-react"

import RichInput from "@/components/custom/share/rich-input"
import { Button } from "@/components/ui/button"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/**
 * 记录页 header: 搜索 + icon 按钮 [刷新 / 清除]。
 * 与代理页的工具栏刻意各自实现 (不共用), 便于两端后续独立演进。
 */
export default function RecordsToolbar({
  query,
  onQueryChange,
  onRefresh,
  onClear,
}: {
  query: string
  onQueryChange: (value: string) => void
  onRefresh: () => void
  onClear: () => void
}) {
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1">
        <RichInput
          prefix={<SearchIcon />}
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={staticValue("contentArea.records.searchPlaceholder")}
          aria-label={staticValue("contentArea.records.searchAria")}
        />
      </div>
      <Button type="button" variant="outline" size="icon" aria-label={staticValue("contentArea.records.refresh")} onClick={onRefresh}>
        <RefreshCwIcon />
      </Button>
      <Button
        type="button"
        variant="outline"
        size="icon"
        aria-label={staticValue("contentArea.records.clear")}
        onClick={onClear}
      >
        <Trash2Icon />
      </Button>
    </div>
  )
}
