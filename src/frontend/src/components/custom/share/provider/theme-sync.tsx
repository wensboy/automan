import * as React from "react"

import { useTheme } from "@/components/custom/share/provider/theme-provider"
import { loadSettings, subscribeSettings } from "@/lib/settings"

/** 设置项 appearence.mode -> 主题 provider 的取值 (auto 即跟随系统)。 */
const MODE_TO_THEME: Record<string, "system" | "light" | "dark"> = {
  auto: "system",
  light: "light",
  dark: "dark",
}

/**
 * 把设置里的 `appearence.mode` 同步到主题 provider —— 设置项才是主题的生效来源。
 *
 * - 启动时读一次后端生效值(引擎三视图叠加结果);
 * - 之后订阅设置事件: 弹窗保存(settings:changed / settings:saved)与外部改文件(settings:reloaded)
 *   都会重新读取并切换, 因此改完设置立刻生效, 外部改动也会回流。
 */
export default function ThemeSync() {
  const { setTheme } = useTheme()

  React.useEffect(() => {
    let cancelled = false

    const apply = () => {
      void loadSettings()
        .then((values) => {
          if (cancelled) return
          const mode = values["appearence.mode"]
          if (typeof mode !== "string") return
          const theme = MODE_TO_THEME[mode]
          if (theme) setTheme(theme)
        })
        .catch(() => undefined)
    }

    apply()
    const off = subscribeSettings(apply)
    return () => {
      cancelled = true
      off()
    }
  }, [setTheme])

  return null
}
