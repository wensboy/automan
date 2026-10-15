import React from "react"
import ReactDOM from "react-dom/client"
import { Toaster } from "sonner"

import App from "@/App"
import StaticProvider from "@/components/custom/share/provider/static-provider"
import ThemeProvider from "@/components/custom/share/provider/theme-provider"
import ThemeSync from "@/components/custom/share/provider/theme-sync"
import { TooltipProvider } from "@/components/ui/tooltip"

import "./index.css"

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ThemeProvider>
      {/* 设置项 appearence.mode -> 主题 (设置是生效来源, provider 只负责落到 <html> 的 .dark) */}
      <ThemeSync />
      <StaticProvider>
        <TooltipProvider>
          <App />
          <Toaster />
        </TooltipProvider>
      </StaticProvider>
    </ThemeProvider>
  </React.StrictMode>,
)
