import * as React from "react"
import { CodeJar } from "codejar"
import { cn } from "cn"
import Prism from "prismjs"

import "prismjs/components/prism-json"

import "./coder.css"

/** Prism 高亮 (JSON): 把编辑区文本重新渲染为带 token class 的 HTML。 */
const highlight = (editor: HTMLElement) => {
  const code = editor.textContent ?? ""
  editor.innerHTML = Prism.highlight(code, Prism.languages.json, "json")
}

/**
 * 代码编辑器 (codejar + Prism): 受控 value + 可滚动区域。
 * codejar 不含 highlighter, 这里接 Prism 的 json 语法; 配色见 coder.css (走 token)。
 */
export default function Coder({
  value,
  onValueChange,
  readOnly = false,
  className,
}: {
  value: string
  onValueChange: (value: string) => void
  readOnly?: boolean
  className?: string
}) {
  const hostRef = React.useRef<HTMLDivElement | null>(null)
  const jarRef = React.useRef<ReturnType<typeof CodeJar> | null>(null)
  const changeRef = React.useRef(onValueChange)
  changeRef.current = onValueChange

  React.useEffect(() => {
    const host = hostRef.current
    if (!host) return

    const jar = CodeJar(host, highlight, { tab: "  ", spellcheck: false })
    jar.onUpdate((code) => changeRef.current(code))
    jarRef.current = jar

    return () => {
      jar.destroy()
      jarRef.current = null
    }
  }, [])

  // 只读态: codejar 本身没有 readOnly 选项, 直接切编辑区的 contenteditable
  // (初始化用的是 "plaintext-only", 恢复可编辑时保持一致)
  React.useEffect(() => {
    const host = hostRef.current
    if (!host) return
    host.contentEditable = readOnly ? "false" : "plaintext-only"
  }, [readOnly])

  // 外部值变化时同步 (内容相同时不动, 避免打断输入与光标)
  React.useEffect(() => {
    const jar = jarRef.current
    if (!jar) return
    if (jar.toString() !== value) {
      jar.updateCode(value)
    }
  }, [value])

  return (
    <div
      ref={hostRef}
      data-slot="coder"
      className={cn(
        "overflow-auto rounded-md border bg-muted/30 p-3 font-mono text-xs leading-relaxed whitespace-pre outline-none",
        className,
      )}
    />
  )
}
