import type { ReactNode } from "react"
import { createHashRouter, Navigate, RouterProvider } from "react-router"

import AuthGuard from "@/components/custom/share/auth-guard"
import AppLayout from "@/layouts/app"
import { hasSession } from "@/lib/auth"
import Automation from "@/pages/automation"
import Capture from "@/pages/capture"
import Home from "@/pages/home"
import Login from "@/pages/login"
import Logs from "@/pages/logs"
import NotFound from "@/pages/not-found"
import Proxy from "@/pages/proxy"
import Records from "@/pages/records"
import Signup from "@/pages/signup"

/** 仅未登录可见 (登录/注册); 已登录直接回主页, 避免重复登录。 */
function GuestOnly({ children }: { children: ReactNode }) {
  return hasSession() ? <Navigate to="/" replace /> : <>{children}</>
}

/**
 * 应用路由表: 登录/注册在守卫之外, 其余挂在 AppLayout 之下并由 AuthGuard 做登录校验。
 * nav-Main 每一项对应一个路由, path 与其 url 一一对应; 面包屑取自当前路由链的
 * handle.crumb (见 layouts/app.tsx), 因此切换 nav-Main 时面包屑同步变化。
 */
const router = createHashRouter([
  {
    path: "/login",
    element: (
      <GuestOnly>
        <Login />
      </GuestOnly>
    ),
  },
  {
    path: "/signup",
    element: (
      <GuestOnly>
        <Signup />
      </GuestOnly>
    ),
  },
  {
    path: "/",
    element: (
      <AuthGuard>
        <AppLayout />
      </AuthGuard>
    ),
    handle: { crumb: "Automan" },
    children: [
      { index: true, element: <Home />, handle: { crumb: "Home" } },
      { path: "proxy", element: <Proxy />, handle: { crumb: "Proxy" } },
      { path: "capture", element: <Capture />, handle: { crumb: "Capture" } },
      { path: "records", element: <Records />, handle: { crumb: "Records" } },
      { path: "logs", element: <Logs />, handle: { crumb: "Logs" } },
      { path: "automation", element: <Automation />, handle: { crumb: "Automation" } },
      { path: "*", element: <NotFound />, handle: { crumb: "404" } },
    ],
  },
])

export default function App() {
  return <RouterProvider router={router} />
}
