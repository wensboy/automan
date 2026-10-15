import type { ReactNode } from "react"

import { InboxIcon } from "lucide-react"

import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/**
 * 页面占位空状态: 统一各页面 body 的"尚未实现 / 无内容"表现 (基于 shadcn 的 Empty 组合)。
 * 传 children 时作为操作区 (EmptyContent), 例如 404 的返回首页按钮。
 */
export default function EmptyState({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children?: ReactNode
}) {
  const { Value } = useStatic()
  return (
    <div className="flex flex-1 items-center justify-center p-6">
      <Empty>
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <InboxIcon />
          </EmptyMedia>
          <EmptyTitle>{title}</EmptyTitle>
          <EmptyDescription>{description ?? Value("contentArea.shared.notImplemented")}</EmptyDescription>
        </EmptyHeader>
        {children && <EmptyContent>{children}</EmptyContent>}
      </Empty>
    </div>
  )
}
