import * as React from "react"
import { PlusIcon } from "lucide-react"

import EmptyState from "@/components/custom/share/empty-state"

import TabBar from "@/components/custom/share/tab-bar"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/**
 * 首页 (nav-Main: Home): header (左标签栏 + 右功能菜单) 与 body 黏合在同一容器内。
 * 首页用于集成非核心能力; 菜单里的每个功能都能作为一个标签显示到 body, 标签栏控制当前显示项。
 * 当前阶段所有功能内容都用空状态占位。
 */
export default function Home() {
  const { Value, icon } = useStatic()
  const features = React.useMemo(
    () =>
      (Value("contentArea.home.features") ?? []).map((feature) => ({
        ...feature,
        Icon: icon(feature.icon),
      })),
    [Value, icon],
  )

  const [openIds, setOpenIds] = React.useState<string[]>([])
  const [activeId, setActiveId] = React.useState("")

  const openFeature = (id: string) => {
    setOpenIds((prev) => (prev.includes(id) ? prev : [...prev, id]))
    setActiveId(id)
  }

  const closeFeature = (id: string) => {
    const next = openIds.filter((item) => item !== id)
    setOpenIds(next)
    if (id === activeId) {
      setActiveId(next[next.length - 1] ?? "")
    }
  }

  const activeFeature = features.find((feature) => feature.id === activeId)
  const openTabs = openIds
    .map((id) => features.find((feature) => feature.id === id))
    .filter((feature): feature is (typeof features)[number] => Boolean(feature))
    .map((feature) => ({ value: feature.id, label: feature.label }))

  return (
    <div className="flex min-h-0 flex-1 flex-col p-4 pt-0">
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border">
        <div className="flex items-center justify-between gap-2 border-b p-1">
          <TabBar
            items={openTabs}
            value={activeId}
            onValueChange={setActiveId}
            onClose={closeFeature}
            className="flex-1"
          />
          <DropdownMenu>
            {/*
              触发链路必须是 Button(asChild) -> DropdownMenuTrigger:
              ui/button 是 React 19 风格普通函数组件(无 forwardRef), 反向嵌套会丢 ref, 菜单被定位到屏幕外。
            */}
            <Button
              asChild
              variant="ghost"
              size="icon"
              className="size-7 shrink-0"
              aria-label={Value("contentArea.home.addFeature")}
            >
              <DropdownMenuTrigger>
                <PlusIcon />
              </DropdownMenuTrigger>
            </Button>
            <DropdownMenuContent align="end" className="w-48">
              {features.map((feature) => (
                <DropdownMenuItem key={feature.id} onSelect={() => openFeature(feature.id)}>
                  <feature.Icon />
                  <span>{feature.label}</span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
        <div className="flex min-h-0 flex-1 flex-col overflow-auto">
          {activeFeature ? (
            <EmptyState
              title={activeFeature.label}
              description={activeFeature.description}
            />
          ) : (
            <EmptyState
              title={Value("contentArea.home.emptyTitle") ?? ""}
              description={Value("contentArea.home.emptyHint")}
            />
          )}
        </div>
      </div>
    </div>
  )
}
