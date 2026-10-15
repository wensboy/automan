import {
  PlayIcon,
  PlusIcon,
  RefreshCwIcon,
  SearchIcon,
  SquareIcon,
  Trash2Icon,
} from "lucide-react"

import RichInput from "@/components/custom/share/rich-input"
import { Button } from "@/components/ui/button"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/** 代理页 header 右段: 搜索 + icon 按钮 [新增 / 刷新 / 清除 / 启动 / 停止] (纯展示, 状态由页面持有)。 */
export default function ProxyToolbar({
  query,
  onQueryChange,
  onAdd,
  onRefresh,
  onClear,
  onStart,
  onStop,
}: {
  query: string
  onQueryChange: (value: string) => void
  onAdd: () => void
  onRefresh: () => void
  onClear: () => void
  onStart: () => void
  onStop: () => void
}) {
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1">
        <RichInput
          prefix={<SearchIcon />}
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={staticValue("contentArea.proxy.searchPlaceholder")}
          aria-label={staticValue("contentArea.proxy.searchAria")}
        />
      </div>
      <Button type="button" variant="outline" size="icon" aria-label={staticValue("contentArea.proxy.add")} onClick={onAdd}>
        <PlusIcon />
      </Button>
      <Button type="button" variant="outline" size="icon" aria-label={staticValue("contentArea.proxy.refresh")} onClick={onRefresh}>
        <RefreshCwIcon />
      </Button>
      <Button
        type="button"
        variant="outline"
        size="icon"
        aria-label={staticValue("contentArea.proxy.clear")}
        onClick={onClear}
      >
        <Trash2Icon />
      </Button>
      <Button type="button" variant="outline" size="icon" aria-label={staticValue("contentArea.proxy.start")} onClick={onStart}>
        <PlayIcon />
      </Button>
      <Button type="button" variant="outline" size="icon" aria-label={staticValue("contentArea.proxy.stop")} onClick={onStop}>
        <SquareIcon />
      </Button>
    </div>
  )
}
