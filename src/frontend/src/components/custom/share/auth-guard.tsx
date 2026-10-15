import type { ReactNode } from "react"
import { Navigate, useLocation } from "react-router"

import { hasSession } from "@/lib/auth"

/**
 * 路由守卫: 包在需要鉴权的路由元素外层, 每次路由切换都重新判定会话,
 * 未登录则重定向到登录页 (子树不会渲染, 避免侧边栏等壳层闪现)。
 */
export default function AuthGuard({ children }: { children: ReactNode }) {
  const location = useLocation()

  if (!hasSession()) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }

  return <>{children}</>
}
