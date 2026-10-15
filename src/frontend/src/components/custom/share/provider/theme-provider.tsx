import * as React from "react"

type Theme = "system" | "light" | "dark"

type ThemeContextValue = {
  theme: Theme
  setTheme: (theme: Theme) => void
}

const STORAGE_KEY = "automan-theme"

const ThemeContext = React.createContext<ThemeContextValue | undefined>(undefined)

function isDark(theme: Theme) {
  if (theme === "dark") return true
  if (theme === "light") return false
  return window.matchMedia("(prefers-color-scheme: dark)").matches
}

/** 主题 provider: 只维护 <html> 上的 .dark 类, 颜色由 index.css 的 token 决定。 */
export default function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = React.useState<Theme>(
    () => (localStorage.getItem(STORAGE_KEY) as Theme | null) ?? "system",
  )

  React.useEffect(() => {
    document.documentElement.classList.toggle("dark", isDark(theme))
    if (theme !== "system") return

    const media = window.matchMedia("(prefers-color-scheme: dark)")
    const onChange = () => document.documentElement.classList.toggle("dark", media.matches)
    media.addEventListener("change", onChange)
    return () => media.removeEventListener("change", onChange)
  }, [theme])

  const value = React.useMemo<ThemeContextValue>(
    () => ({
      theme,
      setTheme: (next) => {
        localStorage.setItem(STORAGE_KEY, next)
        setThemeState(next)
      },
    }),
    [theme],
  )

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}

/** 读取与切换主题 (供后续设置界面使用)。 */
export function useTheme() {
  const value = React.useContext(ThemeContext)
  if (value === undefined) {
    throw new Error("useTheme 必须在 ThemeProvider 内使用")
  }
  return value
}
