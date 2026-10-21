import { Theme as LoadTheme } from "../../bindings/github.com/wensboy/automan/backend/api/web/settingsservice"
import type { Theme } from "../../bindings/github.com/wensboy/automan/backend/service/settings/models"

import { subscribeSettings } from "@/lib/settings"

/**
 * 主题色板的应用层 (theme.json -> CSS 变量)。
 *
 * 数据流: 后端 settings 服务 Theme() 读取 theme.register 指向的文件, 按 theme.current 选主题;
 * 这里把 light/dark 两套 token 写进 <html> 的 inline style(覆盖 index.css 的默认变量)。
 * 监听 settings 事件: 切换主题 / 改 register / 外部改文件后重新拉取并应用。
 */
let palette: Theme | null = null
// 记录已写入的变量: 换主题时清掉旧主题独有、新主题没有的键, 避免残留
const applied = new Set<string>()

function applyTokens(dark: boolean): void {
  if (!palette) return
  const tokens = (dark ? palette.dark : palette.light) ?? {}
  const root = document.documentElement

  for (const key of [...applied]) {
    if (!(key in tokens)) {
      root.style.removeProperty(key)
      applied.delete(key)
    }
  }
  for (const [key, value] of Object.entries(tokens)) {
    if (typeof value !== "string" || value === "") continue
    root.style.setProperty(key, value)
    applied.add(key)
  }
}

/** 拉取当前主题并按明暗应用; 后端不可用时保留 index.css 的默认变量。 */
export async function applyTheme(dark: boolean): Promise<void> {
  try {
    palette = await LoadTheme()
    applyTokens(dark)
  } catch {
    // 启动早期服务未就绪 / 主题文件缺失: 保持默认外观
  }
}

/** 当前生效的主题 id(未加载时为空)。 */
export function currentThemeID(): string {
  return palette?.id ?? ""
}

/**
 * 订阅设置变化后重新应用主题。
 * getDark: 由调用方提供"当前是否暗色"(provider 拥有明暗状态)。
 */
export function bindThemeEvents(getDark: () => boolean): () => void {
  return subscribeSettings((kind) => {
    if (kind === "changed") return
    void applyTheme(getDark())
  })
}
