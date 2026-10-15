import * as React from "react"
import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
} from "@/components/ui/pagination"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/**
 * 记录页 body 尾行 (与表格黏合在同一容器内)。
 * 左 = 当前页行数/总数; 右 = rows-per-page 下拉 + 分页 (前后只有图标, 无文字提示)。
 * 与代理页同款视觉, 但组件独立实现。
 */
export default function RecordsFooter({
  rangeStart,
  rangeEnd,
  total,
  page,
  pageCount,
  pageSizes,
  rowsPerPage,
  onRowsPerPageChange,
  onPageChange,
}: {
  rangeStart: number
  rangeEnd: number
  total: number
  page: number
  pageCount: number
  pageSizes: number[]
  rowsPerPage: number
  onRowsPerPageChange: (size: number) => void
  onPageChange: (page: number) => void
}) {
  // 分页项内部是 <a href="#">, 必须阻止默认行为, 否则 hash 路由会被弹回 "/"。
  const select = (event: React.MouseEvent, next: number) => {
    event.preventDefault()
    onPageChange(Math.min(Math.max(next, 1), pageCount))
  }

  // 只显示局部 3 页 (当前页 + 左右相邻), 避免页码个数随 rows-per-page 变化而伸缩。
  const windowStart = Math.min(Math.max(1, page - 1), Math.max(1, pageCount - 2))
  const windowPages = [windowStart, windowStart + 1, windowStart + 2].filter(
    (value) => value >= 1 && value <= pageCount,
  )

  return (
    <div className="flex items-center justify-between gap-2 border-t px-3 py-2">
      <Badge variant="secondary" className="tabular-nums">
        {total === 0 ? "0/0" : `${rangeStart}-${rangeEnd}/${total}`}
      </Badge>
      <div className="flex items-center gap-2">
        <Select
          value={String(rowsPerPage)}
          onValueChange={(value) => onRowsPerPageChange(Number(value))}
        >
          <SelectTrigger className="h-8 w-20" aria-label={staticValue("contentArea.records.rowsPerPage")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper">
            {pageSizes.map((size) => (
              <SelectItem key={size} value={String(size)}>
                {size}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Pagination className="mx-0 w-auto">
          <PaginationContent>
            <PaginationItem>
              <PaginationLink
                href="#"
                aria-label={staticValue("contentArea.records.previousPage")}
                aria-disabled={page <= 1}
                onClick={(event) => select(event, page - 1)}
              >
                <ChevronLeftIcon className="size-4" />
              </PaginationLink>
            </PaginationItem>
            {windowPages.map((value) => (
              <PaginationItem key={value}>
                <PaginationLink
                  href="#"
                  isActive={value === page}
                  onClick={(event) => select(event, value)}
                >
                  {value}
                </PaginationLink>
              </PaginationItem>
            ))}
            <PaginationItem>
              <PaginationLink
                href="#"
                aria-label={staticValue("contentArea.records.nextPage")}
                aria-disabled={page >= pageCount}
                onClick={(event) => select(event, page + 1)}
              >
                <ChevronRightIcon className="size-4" />
              </PaginationLink>
            </PaginationItem>
          </PaginationContent>
        </Pagination>
      </div>
    </div>
  )
}
