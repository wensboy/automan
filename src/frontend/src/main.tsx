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
import { ensureLogPolling } from "@/store/logs"

import "./index.css"


function NoticeBridge() {
  React.useEffect(() => {
    // 日志拉取是 app 级: 页面没打开也要累积(页面只负责渲染)
    ensureLogPolling()
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
