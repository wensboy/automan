import * as React from "react"
import { toast } from "sonner"

import CaptureFrame from "@/components/custom/capture/frame"
import CaptureSelect from "@/components/custom/capture/select"
import { Running } from "../../../bindings/github.com/wensboy/automan/backend/api/web/proxyservice"
import type { CaptureTarget } from "../../../bindings/github.com/wensboy/automan/backend/service/proxy/models"

/**
 * 抓包 (nav-Main: Capture): header(目标选择) 与 body(内嵌 whistle webui) 黏合在同一容器内。
 * 数据源是 proxy 服务的 Running(): **只含运行中且配置了 uiport 的实例**;
 * 无选中时 body 显示空状态 —— 不做自动选中, 避免"看起来像在抓某个代理"的误解。
 */
export default function Capture() {
  const [targets, setTargets] = React.useState<CaptureTarget[]>([])
  const [selectedId, setSelectedId] = React.useState("")

  const load = React.useCallback(async () => {
    try {
      const list = await Running()
      setTargets(list ?? [])
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error))
    }
  }, [])

  // 首次进入 + 窗口重新获得焦点时刷新(通常在 proxy 页启动/停止代理之后)
  React.useEffect(() => {
    void load()
    const onFocus = () => void load()
    window.addEventListener("focus", onFocus)
    return () => window.removeEventListener("focus", onFocus)
  }, [load])

  // 选中的实例被停止/删除后, 自动回到"未选中"状态
  React.useEffect(() => {
    if (selectedId && !targets.some((item) => item.id === selectedId)) {
      setSelectedId("")
    }
  }, [selectedId, targets])

  const selected = targets.find((item) => item.id === selectedId)

  return (
    <div className="flex min-h-0 flex-1 flex-col p-4 pt-0">
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border">
        <div className="flex items-center gap-2 border-b p-2">
          <CaptureSelect targets={targets} value={selectedId} onValueChange={setSelectedId} />
        </div>
        <CaptureFrame url={selected?.url} />
      </div>
    </div>
  )
}
