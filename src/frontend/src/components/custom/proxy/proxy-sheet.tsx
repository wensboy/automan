import * as React from "react"
import { toast } from "sonner"

import Coder from "@/components/custom/share/coder"
import LabelChunk from "@/components/custom/share/label-chunk"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { proxyConfigTemplate, proxyRowConfig, type ProxyRow } from "@/mock/proxies"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/**
 * 代理编辑 sheet (新增/编辑共用): 标题 -> 单个 Rules 编辑块 -> 底部 [取消][保存]。
 * 传 row 为编辑 (标题 Edit Proxy, 预填该代理信息); 不传为新增 (标题 Add Proxy, 预填模板)。
 * 代理的全部信息都集中在这一个可编辑块内显示; 无关闭按钮; 保存/刷新在 dev 阶段只给反馈。
 */
export default function ProxySheet({
  open,
  onOpenChange,
  row,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  row?: ProxyRow
}) {
  const isEdit = Boolean(row)
  const [rules, setRules] = React.useState(() =>
    row ? proxyRowConfig(row) : proxyConfigTemplate,
  )
  const [rulesEditing, setRulesEditing] = React.useState(true)

  const save = () => {
    onOpenChange(false)
    toast.success((isEdit ? staticValue("contentArea.proxy.updated") : staticValue("contentArea.proxy.saved")) ?? "")
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        showCloseButton={false}
        aria-describedby={undefined}
        className="flex w-full flex-col gap-0 sm:max-w-md"
      >
        <SheetHeader>
          <SheetTitle>
            {isEdit ? staticValue("contentArea.proxy.sheetEdit") : staticValue("contentArea.proxy.sheetAdd")}
          </SheetTitle>
        </SheetHeader>
        <div className="flex min-h-0 flex-1 flex-col px-4">
          <LabelChunk
            label={staticValue("contentArea.proxy.rules") ?? ""}
            className="min-h-0 flex-1"
            editing={rulesEditing}
            onToggleEdit={() => setRulesEditing((prev) => !prev)}
            onSave={() => {
              setRulesEditing(false)
              toast.success(staticValue("contentArea.proxy.rulesSaved") ?? "")
            }}
            onRefresh={() => toast.success(staticValue("contentArea.proxy.rulesRefreshed") ?? "")}
          >
            <Coder
              value={rules}
              onValueChange={setRules}
              readOnly={!rulesEditing}
              className="min-h-0 flex-1 rounded-none border-0 bg-transparent"
            />
          </LabelChunk>
        </div>
        {/* 纯文字按钮: 用 ghost 而非 link —— link 变体自带下划线(hover), 这里只要文字 */}
        <div className="flex items-center justify-end gap-2 p-4">
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            {staticValue("contentArea.proxy.cancel")}
          </Button>
          <Button type="button" variant="ghost" onClick={save}>
            {staticValue("contentArea.proxy.save")}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
