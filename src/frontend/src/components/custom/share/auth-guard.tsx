import * as React from "react"
import type { ReactNode } from "react"
import { Navigate, useLocation } from "react-router"

import { onSessionEvent, restore, storedToken } from "@/lib/auth"

/**
 * 路由守卫: 有本地 token 时先向后端 Resume(按"最后登录时间 + 有效期"判定),
 * 成功才渲染子树; 期间不渲染, 避免壳层闪现。会话在别处结束(登出/到期)时同步退出。
 */
export default function AuthGuard({ children }: { children: ReactNode }) {
  const location = useLocation()
  const [status, setStatus] = React.useState<"checking" | "ready" | "guest">(() =>
    storedToken() ? "checking" : "guest",
  )

  React.useEffect(() => {
    let cancelled = false
    if (storedToken()) {
      void restore().then((user) => {
        if (!cancelled) setStatus(user ? "ready" : "guest")
      })
    }
    const off = onSessionEvent((kind) => {
      if (kind === "logout" || kind === "expired") setStatus("guest")
    })
    return () => {
      cancelled = true
      off()
    }
  }, [])

  if (status === "checking") return null
  if (status === "guest") {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  return <>{children}</>
}
