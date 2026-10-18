import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

import { useStatic } from "@/components/custom/share/provider/static-provider"
import type { CaptureTarget } from "../../../../bindings/github.com/wensboy/automan/backend/service/proxy/models"

/**
 * 抓包目标选择 (header 左段, 不占满整行)。
 * 下拉用 position="popper": 不做"选中项与触发器对齐"的定位, 而是贴在触发器下方展开。
 * 只展示服务端给的可用目标(运行中 + 有 uiport), 过滤逻辑在 proxy 服务里。
 */
export default function CaptureSelect({
  targets,
  value,
  onValueChange,
}: {
  targets: CaptureTarget[]
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
        {targets.map((target) => (
          <SelectItem key={target.id} value={target.id}>
            {target.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
