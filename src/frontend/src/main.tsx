import React from "react"
import ReactDOM from "react-dom/client"

import App from "@/App"
import StaticProvider from "@/components/custom/share/provider/static-provider"
import ThemeProvider from "@/components/custom/share/provider/theme-provider"
import ThemeSync from "@/components/custom/share/provider/theme-sync"
import ErrorBoundary from "@/components/custom/share/error-boundary"
import ThemedToaster from "@/components/custom/share/themed-toaster"
import { TooltipProvider } from "@/components/ui/tooltip"
import { attachNoticeEvents, loadNotices } from "@/lib/notice"

import "./index.css"

/**
 * 运行时提示的接线: **在挂载后**再订阅 —— 模块顶层执行时 Wails 运行时尚未就绪,
 * 早注册的 Events.On 可能收不到事件(表现为"有 toast、抽屉里没有")。
 */
function NoticeBridge() {
  React.useEffect(() => {
    void loadNotices()
    return attachNoticeEvents()
  }, [])
  return null
}

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ThemeProvider>
      {/* 设置项 appearence.mode -> 主题 (设置是生效来源, provider 只负责落到 <html> 的 .dark) */}
      <ThemeSync />
      <StaticProvider>
        <NoticeBridge />
        <TooltipProvider>
          <ErrorBoundary>
            <App />
          </ErrorBoundary>
          <ThemedToaster />
        </TooltipProvider>
      </StaticProvider>
    </ThemeProvider>
  </React.StrictMode>,
)
