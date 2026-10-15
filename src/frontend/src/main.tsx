import React from "react"
import ReactDOM from "react-dom/client"
import { Toaster } from "sonner"

import App from "@/App"
import StaticProvider from "@/components/custom/share/provider/static-provider"
import ThemeProvider from "@/components/custom/share/provider/theme-provider"
import { TooltipProvider } from "@/components/ui/tooltip"

import "./index.css"

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ThemeProvider>
      <StaticProvider>
        <TooltipProvider>
          <App />
          <Toaster />
        </TooltipProvider>
      </StaticProvider>
    </ThemeProvider>
  </React.StrictMode>,
)
