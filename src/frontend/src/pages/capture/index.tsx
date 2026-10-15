import * as React from "react"

import CaptureFrame from "@/components/custom/capture/frame"
import CaptureSelect from "@/components/custom/capture/select"
import { proxies } from "@/mock/proxies"

/**
 * 抓包 (nav-Main: Capture): header (目标选择) 与 body (内嵌网页) 黏合在同一容器内。
 * 仅当选中项"运行中且带 web 端点"时才渲染网页, 否则 body 显示空状态提示。
 */
export default function Capture() {
  const [selectedId, setSelectedId] = React.useState("")

  const selected = proxies.find((row) => row.id === selectedId)
  const endpoint = selected && selected.status === "running" ? selected.endpoint : undefined

  return (
    <div className="flex min-h-0 flex-1 flex-col p-4 pt-0">
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border">
        <div className="flex items-center gap-2 border-b p-2">
          <CaptureSelect
            proxies={proxies}
            value={selectedId}
            onValueChange={setSelectedId}
          />
        </div>
        <CaptureFrame endpoint={endpoint} />
      </div>
    </div>
  )
}
