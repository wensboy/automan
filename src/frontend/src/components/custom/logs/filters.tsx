import { SearchIcon } from "lucide-react"
import { cn } from "cn"

import RichInput from "@/components/custom/share/rich-input"

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { LogLevel, LogTimeRange } from "@/components/custom/logs/model"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/** 日志页头部: 搜索 + 级别 + 时间范围 (纯展示, 状态由页面持有)。 */
export default function LogFilters({
  className,
  query,
  onQueryChange,
  level,
  onLevelChange,
  range,
  onRangeChange,
}: {
  className?: string
  query: string
  onQueryChange: (value: string) => void
  level: LogLevel | "all"
  onLevelChange: (value: LogLevel | "all") => void
  range: LogTimeRange
  onRangeChange: (value: LogTimeRange) => void
}) {
  const { Value } = useStatic()

  return (
    <div className={cn("flex items-center gap-2 p-2", className)}>
      <div className="flex-1">
        <RichInput
          prefix={<SearchIcon />}
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={Value("contentArea.logs.searchPlaceholder")}
          aria-label={Value("contentArea.logs.searchAria")}
        />
      </div>
      <Select
        value={level}
        onValueChange={(value) => onLevelChange(value as LogLevel | "all")}
      >
        <SelectTrigger className="w-32" aria-label={Value("contentArea.logs.levelAria")}>
          <SelectValue placeholder={Value("contentArea.logs.levelPlaceholder")} />
        </SelectTrigger>
        <SelectContent position="popper">
          <SelectItem value="all">{Value("contentArea.logs.allLevels")}</SelectItem>
          {(Value("contentArea.logs.levels") ?? []).map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select
        value={range}
        onValueChange={(value) => onRangeChange(value as LogTimeRange)}
      >
        <SelectTrigger className="w-36" aria-label={Value("contentArea.logs.timeAria")}>
          <SelectValue placeholder={Value("contentArea.logs.timePlaceholder")} />
        </SelectTrigger>
        <SelectContent position="popper">
          {(Value("contentArea.logs.timeRanges") ?? []).map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
