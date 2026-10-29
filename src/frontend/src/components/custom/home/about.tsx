import { Browser } from "@wailsio/runtime"
import { ExternalLinkIcon } from "lucide-react"
import { useEffect, useState } from "react"

import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { aboutLinks, loadAbout, type AboutModel } from "@/lib/about"
import { resolveIcon } from "@/lib/icons"

/**
 * Home > About: 程序信息(见 docs/design/v25.md §17)。
 *
 * 组件只负责渲染: 取值/映射/格式化全部在 lib/about.ts; 后端是唯一数据来源。
 */
export default function AboutView() {
  const [model, setModel] = useState<AboutModel | null>(null)

  useEffect(() => {
    let alive = true
    void (async () => {
      try {
        const next = await loadAbout()
        if (alive) setModel(next)
      } catch {
        // 后端不可用时保持空(页面显示占位), 不做本地兜底
      }
    })()
    return () => {
      alive = false
    }
  }, [])

  const links = aboutLinks(model?.repositoryUrl ?? "")

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-auto p-4">
      {/* 垂直居中用 my-auto 而非父层 justify-center: 内容超高时 auto margin 归零, 不会把顶部裁掉 */}
      <div className="my-auto flex w-full flex-col items-center">
        <img src="/appicon.png" alt={model?.appName ?? "Automan"} className="size-24 rounded-2xl shadow-sm ring-1 ring-border/60 sm:size-28 lg:size-32" />

        <Card className="mt-6 w-full gap-0 py-0">
          <CardContent className="px-4 py-4">
            <div className="text-center">
              <h2 className="text-base font-semibold">{model?.appName ?? "-"}</h2>
              <p className="mt-1 text-xs text-muted-foreground">{model?.description ?? ""}</p>
            </div>

            <dl className="mt-4 space-y-2 text-xs">
              {(model?.rows ?? []).map((row) => (
                <div key={row.key} className="flex items-center justify-between gap-4">
                  <dt className="text-muted-foreground">{row.label}</dt>
                  <dd className="min-w-0 truncate text-right">
                    {row.href ? (
                      <button
                        type="button"
                        className="inline-flex max-w-full items-center gap-1 text-primary hover:underline"
                        title={row.value}
                        onClick={() => void Browser.OpenURL(row.href as string)}
                      >
                        <span className="truncate">{row.value}</span>
                        <ExternalLinkIcon className="size-3 shrink-0" />
                      </button>
                    ) : (
                      <span title={row.value}>{row.value}</span>
                    )}
                  </dd>
                </div>
              ))}
            </dl>
          </CardContent>
        </Card>

        <div className="mt-4 grid w-full grid-cols-2 gap-2">
          {(model?.actions ?? []).map((action) => {
            const Icon = resolveIcon(action.icon)
            const href = action.id === "coffee" ? links.sponsor : links.releases
            return (
              <Button
                key={action.id}
                type="button"
                variant="outline"
                className="w-full gap-2"
                disabled={!href}
                onClick={() => href && void Browser.OpenURL(href)}
              >
                <Icon className="size-4" />
                <span>{action.label}</span>
              </Button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
