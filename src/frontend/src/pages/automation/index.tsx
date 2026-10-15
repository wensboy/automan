import * as React from "react"

import EmptyState from "@/components/custom/share/empty-state"

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/** 自动化页签: 创建 (New) 与 管理 (Workflow)。 */
const AUTOMATION_TABS = ["new", "workflow"] as const

/**
 * 自动化 (nav-Main: Automation): 自动化相关操作 (创建 / 管理)。
 * 与抓包页同构 —— header (页签) 与 body 黏合在同一容器内, body 跟随页签切换; 当前用空状态占位。
 */
export default function Automation() {
  const { Value } = useStatic()
  const [tab, setTab] = React.useState<string>(AUTOMATION_TABS[0])

  return (
    <div className="flex min-h-0 flex-1 flex-col p-4 pt-0">
      <Tabs
        value={tab}
        onValueChange={setTab}
        // gap-0: 抵消 ui/tabs 自带的 gap-2, 让 header 与 body 黏合 (其余样式沿用组件默认)
        className="flex min-h-0 flex-1 flex-col gap-0 overflow-hidden rounded-lg border"
      >
        <div className="flex items-center gap-2 border-b p-2">
          <TabsList>
            {AUTOMATION_TABS.map((item) => (
              <TabsTrigger key={item} value={item}>
                {item === "new"
                  ? Value("contentArea.automation.tabNew")
                  : Value("contentArea.automation.tabWorkflow")}
              </TabsTrigger>
            ))}
          </TabsList>
        </div>
        {AUTOMATION_TABS.map((item) => (
          <TabsContent
            key={item}
            value={item}
            className="flex min-h-0 flex-1 flex-col overflow-auto"
          >
            <EmptyState
              title={
                (item === "new"
                  ? Value("contentArea.automation.tabNew")
                  : Value("contentArea.automation.tabWorkflow")) ?? ""
              }
              description={Value("contentArea.automation.empty")}
            />
          </TabsContent>
        ))}
      </Tabs>
    </div>
  )
}
