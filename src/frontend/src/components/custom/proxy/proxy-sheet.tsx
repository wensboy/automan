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
import { emptyInput, prettyJSON, targetOf, toInput, type Input, type Proxy } from "@/components/custom/proxy/model"

/** 新增时的草稿: rules 取后端模板(data/default/proxy.rules.json), 取不到则用空对象。 */
function buildDraft(template?: string): Input {
  const blank = emptyInput()
  return template ? { ...blank, rules: prettyJSON(template) } : blank
}

/**
 * 代理编辑 sheet (新增/编辑共用): 标题 -> Name/Target 输入 + 单个 Rules 编辑块 -> 底部 [取消][保存]。
 * 传 row 为编辑 (标题 Edit Proxy, 预填该行); 不传为新增。
 * 保存交给调用方的 onSubmit: 真实落库(v17 §8), 失败时把后端校验信息直接反馈出来。
 */
export default function ProxySheet({
  open,
  onOpenChange,
  row,
  rulesTemplate,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  row?: Proxy
  /** 新建时的 rules 初始内容(后端 data/default/proxy.rules.json) */
  rulesTemplate?: string
  onSubmit: (input: Input, id?: string) => Promise<void>
}) {
  const isEdit = Boolean(row)
  const [draft, setDraft] = React.useState<Input>(() => (row ? toInput(row) : buildDraft(rulesTemplate)))
  const [rulesEditing, setRulesEditing] = React.useState(true)

  // 本组件**常驻挂载**(open 只是开关), 因此 useState 的初值早于模板到达,
  // 且关闭再打开不会重建 state。这里在每次打开"新增"时按最新模板重置草稿。
  React.useEffect(() => {
    if (!open || row) return
    setDraft(buildDraft(rulesTemplate))
    setRulesEditing(true)
  }, [open, row, rulesTemplate])
  const [pending, setPending] = React.useState(false)
  // 有效 rules: 新增时草稿为空(或仍是空对象)则回落到模板 —— 不依赖初始化时序
  const effectiveRules = React.useMemo(() => {
    if (isEdit) return draft.rules
    const current = draft.rules.trim()
    if (current === "" || current === "{}") return rulesTemplate ?? current
    return draft.rules
  }, [draft.rules, isEdit, rulesTemplate])

  const submit = async () => {
    setPending(true)
    try {
      await onSubmit({ ...draft, rules: effectiveRules }, row?.id)
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
          {/* target 由 rules 的 host + port 派生: 这里只展示, 不提供编辑(库里仍冗余存储该列) */}
          <LabelInput
            label={staticValue("contentArea.proxy.columns.target") ?? ""}
            value={targetOf(effectiveRules)}
            readOnly
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
              value={effectiveRules}
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
