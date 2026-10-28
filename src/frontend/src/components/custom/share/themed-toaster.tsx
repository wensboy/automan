import * as React from "react"
import { Toaster } from "sonner"

import { useTheme } from "@/components/custom/share/provider/theme-provider"

/**
 * Toast 跟随主题: 颜色全部取 **CSS 变量**(bg-popover / text-popover-foreground / border),
 * 因此 theme.json 换色、亮暗切换都会同步生效(而不是 sonner 自带的固定色)。
 */
function useIsDark(): boolean {
  const { theme } = useTheme()
  const [systemDark, setSystemDark] = React.useState(
    () => window.matchMedia("(prefers-color-scheme: dark)").matches,
  )

  React.useEffect(() => {
    if (theme !== "system") return
    const media = window.matchMedia("(prefers-color-scheme: dark)")
    const onChange = (): void => setSystemDark(media.matches)
    setSystemDark(media.matches)
    media.addEventListener("change", onChange)
    return () => media.removeEventListener("change", onChange)
  }, [theme])

  if (theme === "dark") return true
  if (theme === "light") return false
  return systemDark
}

export default function ThemedToaster() {
  const dark = useIsDark()
  return (
    <Toaster
      theme={dark ? "dark" : "light"}
      toastOptions={{
        // 用主题 token 覆盖 sonner 的默认配色(优先级高于其内置样式)
        style: {
          background: "var(--popover)",
          color: "var(--popover-foreground)",
          border: "1px solid var(--border)",
          borderRadius: "var(--radius)",
          boxShadow: "0 8px 24px oklch(0 0 0 / 0.12)",
        },
        classNames: {
          toast: "!min-w-0 !max-w-[min(92vw,380px)]",
          // 标题是主信息: 允许多行完整展示(长错误信息不再横向溢出)
          title: "!whitespace-normal !break-words !leading-snug",
          // 细节内容保持截断, 避免 toast 过高
          description: "!text-muted-foreground !line-clamp-2 !break-words",
          actionButton: "!bg-primary !text-primary-foreground",
          cancelButton: "!bg-muted !text-muted-foreground",
        },
      }}
    />
  )
}
