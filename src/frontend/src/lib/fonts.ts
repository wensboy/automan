import { Font } from "../../bindings/github.com/wensboy/automan/backend/api/web/settingsservice"

/**
 * 字体栈加载(按 id 取 path)。
 *
 * 约定(见 docs/design/v23.md §43):
 *   - 候选项永远来自后端 settings.Options("appearence.font.options") —— 由 font.json 决定;
 *   - 前端只负责"针对当前选中的 id 取 path 并热替换", 不硬编码字体列表。
 */
const fallbackStacks: Record<string, string> = {
  inter: '"Inter Variable", ui-sans-serif, system-ui, sans-serif',
  "jetbrains-mono": '"JetBrains Mono Variable", ui-monospace, SFMono-Regular, monospace',
  system: 'ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, "Noto Sans", Arial, sans-serif',
}

/** 按 id 取字体栈(后端不可用时用内置兜底, 未知 id 回落 inter)。 */
export async function loadFontStack(id?: string): Promise<string> {
  const key = id ?? "inter"
  try {
    const entry = await Font(key)
    if (entry?.path) return entry.path
  } catch {
    // 启动早期服务未就绪 / 文件缺失: 用兜底
  }
  return fallbackStacks[key] ?? fallbackStacks.inter
}
