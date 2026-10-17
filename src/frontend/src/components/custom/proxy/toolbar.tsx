import {
  PlayIcon,
  PlusIcon,
  RefreshCwIcon,
  SearchIcon,
  SquareIcon,
  Trash2Icon,
} from "lucide-react"

import { staticValue } from "@/components/custom/share/provider/static-provider"
import RichInput from "@/components/custom/share/rich-input"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { PROXY_STATUSES, type ProxyStatusFilter } from "@/components/custom/proxy/model"

/**
 * 代理页 header 右段: 搜索 + 状态过滤 + icon 按钮 [新增 / 刷新 / 清除 / 启动 / 停止]
 * (纯展示, 状态与行为由页面持有)。
 */
export default function ProxyToolbar({
  query,
  onQueryChange,
  status,
  onStatusChange,
  onAdd,
  onRefresh,
  onClear,
  onStart,
  onStop,
}: {
  query: string
  onQueryChange: (value: string) => void
  status: ProxyStatusFilter
  onStatusChange: (value: ProxyStatusFilter) => void
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
      <Select value={status} onValueChange={(value) => onStatusChange(value as ProxyStatusFilter)}>
        <SelectTrigger className="w-40" aria-label={staticValue("contentArea.proxy.statusAria")}>
          <SelectValue />
        </SelectTrigger>
        {/* position="popper": 与本地其它 select 一致(非触发器对齐) */}
        <SelectContent position="popper">
          <SelectItem value="all">{staticValue("contentArea.proxy.allStatuses")}</SelectItem>
          {PROXY_STATUSES.map((item) => (
            <SelectItem key={item} value={item}>
              {staticValue(`contentArea.proxy.statusLabel.${item}`) as string}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
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
