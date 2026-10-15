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
import { recordRowContent, type RecordRow } from "@/mock/records"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/**
 * 记录编辑 sheet: 标题 -> 单个 Content 编辑块 -> 底部 [取消][保存]。
 * 记录页没有"新增"入口, 因此这里只处理编辑 (由页面在选中行时才挂载)。
 * 与代理页的 sheet 刻意各自实现; 保存/刷新在 dev 阶段只给反馈。
 */
export default function RecordSheet({
  open,
  onOpenChange,
  row,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  row: RecordRow
}) {
  const [content, setContent] = React.useState(() => recordRowContent(row))
  const [editing, setEditing] = React.useState(true)

  const save = () => {
    onOpenChange(false)
    toast.success(staticValue("contentArea.records.saved") ?? "")
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
          <SheetTitle>{staticValue("contentArea.records.sheetEdit")}</SheetTitle>
        </SheetHeader>
        <div className="flex min-h-0 flex-1 flex-col px-4">
          <LabelChunk
            label={staticValue("contentArea.records.content") ?? ""}
            className="min-h-0 flex-1"
            editing={editing}
            onToggleEdit={() => setEditing((prev) => !prev)}
            onSave={() => {
              setEditing(false)
              toast.success(staticValue("contentArea.records.contentSaved") ?? "")
            }}
            onRefresh={() => toast.success(staticValue("contentArea.records.contentRefreshed") ?? "")}
          >
            <Coder
              value={content}
              onValueChange={setContent}
              readOnly={!editing}
              className="min-h-0 flex-1 rounded-none border-0 bg-transparent"
            />
          </LabelChunk>
        </div>
        {/* 纯文字按钮: 用 ghost 而非 link —— link 变体自带下划线(hover), 这里只要文字 */}
        <div className="flex items-center justify-end gap-2 p-4">
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            {staticValue("contentArea.records.cancel")}
          </Button>
          <Button type="button" variant="ghost" onClick={save}>
            {staticValue("contentArea.records.save")}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
