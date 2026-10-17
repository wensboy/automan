import * as React from "react"
import { toast } from "sonner"

import Coder from "@/components/custom/share/coder"
import LabelChunk from "@/components/custom/share/label-chunk"
import LabelInput from "@/components/custom/share/label-input"
import { staticValue } from "@/components/custom/share/provider/static-provider"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { emptyInput, toInput, type Input, type Proxy } from "@/components/custom/proxy/model"

/**
 * 代理编辑 sheet (新增/编辑共用): 标题 -> Name/Target 输入 + 单个 Rules 编辑块 -> 底部 [取消][保存]。
 * 传 row 为编辑 (标题 Edit Proxy, 预填该行); 不传为新增。
 * 保存交给调用方的 onSubmit: 真实落库(v17 §8), 失败时把后端校验信息直接反馈出来。
 */
export default function ProxySheet({
  open,
  onOpenChange,
  row,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  row?: Proxy
  onSubmit: (input: Input, id?: string) => Promise<void>
}) {
  const isEdit = Boolean(row)
  const [draft, setDraft] = React.useState<Input>(() => (row ? toInput(row) : emptyInput()))
  const [rulesEditing, setRulesEditing] = React.useState(true)
  const [pending, setPending] = React.useState(false)

  const submit = async () => {
    setPending(true)
    try {
      await onSubmit(draft, row?.id)
      onOpenChange(false)
    } catch (error) {
      // 后端会给出明确原因(必填/重复名/枚举越界等), 原样呈现
      toast.error(error instanceof Error ? error.message : String(error))
    } finally {
      setPending(false)
    }
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
        <div className="flex min-h-0 flex-1 flex-col gap-2 px-4">
          <LabelInput
            label={staticValue("contentArea.proxy.columns.name") ?? ""}
            value={draft.name}
            onChange={(event) => setDraft((prev) => ({ ...prev, name: event.target.value }))}
          />
          <LabelInput
            label={staticValue("contentArea.proxy.columns.target") ?? ""}
            value={draft.target}
            onChange={(event) => setDraft((prev) => ({ ...prev, target: event.target.value }))}
          />
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
              value={draft.rules}
              onValueChange={(value) => setDraft((prev) => ({ ...prev, rules: value }))}
              readOnly={!rulesEditing}
              className="min-h-0 flex-1 rounded-none border-0 bg-transparent"
            />
          </LabelChunk>
        </div>
        {/* 纯文字按钮: 用 ghost 而非 link —— link 变体自带下划线(hover), 这里只要文字 */}
        <div className="flex items-center justify-end gap-2 p-4">
          <Button type="button" variant="ghost" disabled={pending} onClick={() => onOpenChange(false)}>
            {staticValue("contentArea.proxy.cancel")}
          </Button>
          <Button type="button" variant="ghost" disabled={pending} onClick={submit}>
            {staticValue("contentArea.proxy.save")}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
