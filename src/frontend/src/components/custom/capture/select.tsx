import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

import type { ProxyRow } from "@/mock/proxies"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/**
 * 抓包目标选择 (header 左段, 不占满整行)。
 * 下拉用 position="popper": 不做"选中项与触发器对齐"的定位, 而是贴在触发器下方展开。
 */
export default function CaptureSelect({
  proxies,
  value,
  onValueChange,
}: {
  proxies: ProxyRow[]
  value: string
  onValueChange: (value: string) => void
}) {
  const { Value } = useStatic()

  return (
    <Select value={value} onValueChange={onValueChange}>
      <SelectTrigger className="w-56" aria-label={Value("contentArea.capture.targetAria")}>
        <SelectValue placeholder={Value("contentArea.capture.selectPlaceholder")} />
      </SelectTrigger>
      <SelectContent position="popper">
        {proxies.map((row) => (
          <SelectItem key={row.id} value={row.id}>
            {row.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
