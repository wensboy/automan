import type { ComponentProps } from "react"

import { Button } from "@/components/ui/button"

/**
 * 开关态按钮: 常规 Button 封装, 用色调表达选中态 (选中 = 实心 primary, 未选中 = outline)。
 * 接口与 Button 一致, 额外接受 active / onToggle; variant/size 仍可覆盖。
 */
export default function SwitchButton({
  active,
  onToggle,
  ...props
}: ComponentProps<typeof Button> & {
  active: boolean
  onToggle?: () => void
}) {
  return (
    <Button
      type="button"
      variant={active ? "default" : "outline"}
      size="icon"
      aria-pressed={active}
      data-state={active ? "on" : "off"}
      {...props}
      onClick={onToggle}
    />
  )
}
