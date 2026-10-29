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

  // 宽度跟随容器: 滚动条全仓隐藏后, 若给 iframe 垫最小宽度, 外层横向滚动只能靠滚轮,
  // 而指针落在 iframe 上时滚轮事件被其文档吃掉, 溢出的部分将无法触达 —— 故不再做宽度底座。
  // 内嵌页自身的横向溢出(跨域文档)由它自己的文档滚动承担, 这里只保证盒子有界不撑开外壳。
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
