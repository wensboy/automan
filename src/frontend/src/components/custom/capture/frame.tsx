import EmptyState from "@/components/custom/share/empty-state"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/**
 * 抓包内容区: 有可用端点时内嵌网页 (iframe, 后续指向 whistle 端点), 否则显示空状态。
 * 有效性判断 (运行中 + 有端点) 由页面决定, 这里只按 endpoint 是否存在分支。
 */
export default function CaptureFrame({ endpoint }: { endpoint?: string }) {
  const { Value } = useStatic()

  if (!endpoint) {
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
      src={endpoint}
      title={Value("contentArea.capture.frameTitle") ?? ""}
      className="min-h-0 w-full flex-1 border-0 bg-background"
    />
  )
}
