import * as React from "react"

import type { LucideIcon } from "lucide-react"

import { resolveIcon } from "@/lib/icons"
import { staticStore } from "@/lib/static-data"
import type { StaticPatch, StaticValue, StaticView } from "@/static/types/static"
import type { SidebarStatic } from "@/static/types/sidebar"


/**
 * 静态数据 provider (layer view):
 * - 数据来自 `static/default/*.json`(base) 与 `static/cover/*.json`(cover), 由 `lib/static-data.ts` 组装;
 * - 对外提供层视图: `Value(jsonKey)` 取值, `shadow(patch)` 临时变更(返回新视图), `cover(patch)` 永久变更;
 * - 可通过 `<StaticProvider shadow={…}>` 给某棵子树一个局部视图 (只影响该子树内的读取)。
 *
 * 兼容说明: 现有组件仍通过 `useStatic().sidebar` 读取解析后的数据, 该字段暂时保留;
 * 待按层视图改造消费端后可移除 (见 docs/design/v14.md §2「静态数据」)。
 */

/** 图标名 (kebab-case, 与 json 对齐) -> 组件: 显式登记, 只打包用到的图标。 */
/** 原始数据 -> 消费端形态 (图标名解析为组件)。 */
function parseSidebar(raw: StaticValue<"sidebar"> | undefined): SidebarStatic {
  if (!raw) {
    return { teams: [], navMain: [], navSecondary: [], user: { name: "", email: "", avatar: "" } }
  }
  return {
    teams: raw.teams.map((team) => ({
      name: team.name,
      plan: team.plan,
      logo: resolveIcon(team.logo),
    })),
    navMain: raw.navMain.map((item) => ({
      title: item.title,
      url: item.url,
      icon: resolveIcon(item.icon),
    })),
    navSecondary: raw.navSecondary.map((item) => ({
      title: item.title,
      url: item.url,
      icon: resolveIcon(item.icon),
    })),
    user: raw.user,
  }
}

/** 层视图 + 兼容字段 (sidebar 待改造完成后移除)。 */
export type Statics = StaticView & {
  sidebar: SidebarStatic
  /** 图标名 -> 组件 (静态数据只存名字, 由这里解析)。 */
  icon: (name: string) => LucideIcon
}

const StaticContext = React.createContext<Statics | undefined>(undefined)

export default function StaticProvider({
  shadow,
  children,
}: {
  /** 给该子树一个局部视图 (临时变更, 不修改数据)。 */
  shadow?: StaticPatch
  children: React.ReactNode
}) {
  // 订阅 cover(): 永久变更后重新渲染, 让消费者看到新值
  React.useSyncExternalStore(staticStore.subscribe, staticStore.snapshot, staticStore.snapshot)

  const view = React.useMemo(
    () => (shadow ? staticStore.view().shadow(shadow) : staticStore.view()),
    [shadow],
  )

  const value = React.useMemo<Statics>(
    () => ({
      ...view,
      sidebar: parseSidebar(view.Value("sidebar")),
      icon: resolveIcon,
    }),
    [view],
  )

  return <StaticContext.Provider value={value}>{children}</StaticContext.Provider>
}

/**
 * 模块级读取: 与 `useStatic()` 共用同一个静态数据容器, 取值为纯文本替换提供方便
 * (无 hook 约束)。注意它不感知 `<StaticProvider shadow>` 的子树作用域 ——
 * 组件内文案若需要被子树 shadow 影响, 请改用 `useStatic()`。
 */
export function staticValue<P extends string>(path: P): StaticValue<P> | undefined {
  return staticStore.view().Value(path)
}

/** 模块级图标解析 (静态数据里只存名字)。 */
export function staticIcon(name: string): LucideIcon {
  return resolveIcon(name)
}

/** 读取静态数据层视图 (必须在 StaticProvider 内使用)。 */
export function useStatic(): Statics {
  const value = React.useContext(StaticContext)
  if (value === undefined) {
    throw new Error("useStatic 必须在 StaticProvider 内使用")
  }
  return value
}
