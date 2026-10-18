import EmptyState from "@/components/custom/share/empty-state"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/**
 * 抓包内容区: 有 webui 地址时内嵌 (iframe), 否则显示空状态。
 * 地址由 proxy 服务给出(http://<host>:<uiport>), 这里只按 url 是否存在分支。
 */
export default function CaptureFrame({ url }: { url?: string }) {
  const { Value } = useStatic()

  if (!url) {
    // EmptyState 自身是 flex-1 + 居中的容器, 直接作为 body 的伸缩子项即可
    return (
      <EmptyState
        title={Value("contentArea.capture.emptyTitle") ?? ""}
        description={Value("contentArea.capture.emptyHint")}
      />
    )
  }

  return (
    <iframe
      // key 让切换目标时重建 iframe(否则部分 webview 会沿用上一个页面)
      key={url}
      src={url}
      title={Value("contentArea.capture.frameTitle") ?? ""}
      className="min-h-0 w-full flex-1 border-0 bg-background"
    />
  )
}
