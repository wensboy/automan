import * as React from "react"

import { Button } from "@/components/ui/button"

type State = { error: Error | null }

/**
 * 顶层错误边界: 渲染期异常时显示可读信息 + 重试, 而不是留下**纯黑/空白**的窗口。
 * (Wails 下没有浏览器默认的错误页, React 抛错后组件树会被卸载, 视觉上就是"黑屏")
 */
export default class ErrorBoundary extends React.Component<{ children: React.ReactNode }, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error): void {
    console.error("[ui] render failed:", error)
  }

  render(): React.ReactNode {
    if (this.state.error) {
      return (
        <div className="flex h-dvh flex-col items-center justify-center gap-3 p-6 text-center">
          <p className="text-sm font-medium">界面渲染出错</p>
          <p className="max-w-md text-xs text-muted-foreground">{this.state.error.message}</p>
          <Button size="sm" variant="outline" onClick={() => this.setState({ error: null })}>
            重试
          </Button>
        </div>
      )
    }
    return this.props.children
  }
}
