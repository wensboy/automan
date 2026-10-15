import type { ElementType } from "react"
import type { LucideIcon } from "lucide-react"

/**
 * 侧边栏静态数据的"已解析"形态 (static/sidebar.json 经 provider 解析图标后的结果)。
 * 多个组件复用这些类型, 故独立于 static/types/ 下。
 */

export type SidebarTeam = {
  name: string
  plan: string
  logo: ElementType
}

export type SidebarNavItem = {
  title: string
  url: string
  icon?: LucideIcon
}

export type SidebarSecondaryItem = {
  title: string
  url: string
  icon: LucideIcon
}

export type SidebarUser = {
  name: string
  email: string
  avatar: string
}

export type SidebarStatic = {
  teams: SidebarTeam[]
  navMain: SidebarNavItem[]
  navSecondary: SidebarSecondaryItem[]
  user: SidebarUser
}
