import * as React from "react"
import { ChevronRightIcon } from "lucide-react"

import { cn } from "cn"
import { toast } from "sonner"

import ListInput from "@/components/custom/share/list-input"
import { useStatic } from "@/components/custom/share/provider/static-provider"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
} from "@/components/ui/sidebar"
import { Slider } from "@/components/ui/slider"
import { Switch } from "@/components/ui/switch"
import {
  loadSettings,
  parseFieldValue,
  saveSettings,
  rangeBounds,
  settingsGroups,
  subscribeSettings,
} from "@/lib/settings"
import {
  parseFieldType,
  type SettingsField,
  type SettingsFieldEntry,
  type SettingsScalarType,
  type SettingsValue,
  type SettingsValues,
} from "@/static/types/settings"

/** 元素文本 <-> 标量 (list-input 只处理文本, 这里按 <T> 转换)。 */
function fromText(text: string, of: SettingsScalarType): SettingsValue | undefined {
  const trimmed = text.trim()
  if (trimmed === "") return undefined
  if (of === "number") {
    const value = Number(trimmed)
    return Number.isNaN(value) ? undefined : value
  }
  if (of === "boolean") {
    if (trimmed === "true") return true
    if (trimmed === "false") return false
    return undefined
  }
  return trimmed
}

/**
 * 单个配置项: 按 `type` 选择控件 ——
 * `enum<T>`->select(popper 非对齐) / `bool`->switch / `string`->field /
 * `number`->slider(range 为 [min,max] 时)或数字输入 / `array<T>`->list-input。
 * 值缺失时显示占位符: 前端不提供默认值, 生效值一律由运行时下发。\n * label 一律原样显示(嵌套层级由 settings.json 的 label 自己写成 `session>expiration` 这类形式);
 * 分隔线只在 depth=1 的项之间(`trail` 只用于判断深度, 不再拼进 label)。
 */
function SettingsRow({
  field,
  separator,
  value,
  onChange,
}: {
  field: SettingsFieldEntry
  /** 是否显示与下一项的分隔线: 只有 depth=1 的项才分隔(见父层注释)。 */
  separator: boolean
  value: SettingsValue | undefined
  onChange: (next: SettingsValue) => void
}) {
  const type = parseFieldType(field.type)
  const missing = value === undefined

  return (
    <div className={cn("flex flex-col gap-1.5 py-2.5", separator && "border-b")}>
      <div className="flex items-center justify-between gap-3">
        <Label htmlFor={field.key} className="flex items-center gap-1 text-sm font-medium">
          {/*
            层级: label 里每个 `.` 都渲染成 chevron-right 图标 —— 比 `>` 字形在不同字体下更稳定,
            间距也一致(gap 控制)。其余文本原样显示; 只按 `.` 切分, 不做其它加工。
          */}
          {field.label.split(".").map((part, index) => (
            <React.Fragment key={`${part}-${index}`}>
              {index > 0 && (
                <ChevronRightIcon aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
              )}
              <span>{part.trim()}</span>
            </React.Fragment>
          ))}
        </Label>
        {!missing && type?.kind === "bool" && (
          <Switch
            id={field.key}
            checked={Boolean(value)}
            onCheckedChange={(checked) => onChange(checked)}
          />
        )}
        {/* 数值只在滑杆形式下显示: 手动输入时 input 内已能看到, 不重复占位 */}
        {!missing && type?.kind === "number" && isRanged(field) && (
          <span className="text-muted-foreground text-xs tabular-nums">{Number(value)}</span>
        )}
        {missing && <span className="text-muted-foreground text-xs">—</span>}
      </div>

      {!missing && type?.kind === "enum" && (
        <Select value={String(value)} onValueChange={(next) => onChange(next)}>
          <SelectTrigger id={field.key} className="h-8 w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper">
            {(field.type.startsWith("enum") ? (field as { options: SettingsValue[] }).options : []).map(
              (option) => (
                <SelectItem key={String(option)} value={String(option)}>
                  {String(option)}
                </SelectItem>
              ),
            )}
          </SelectContent>
        </Select>
      )}

      {!missing && type?.kind === "string" && (
        <Input
          id={field.key}
          value={String(value)}
          onChange={(event) => onChange(event.target.value)}
          className="h-8"
        />
      )}

      {!missing && type?.kind === "number" && <NumberControl field={field} value={Number(value)} onChange={onChange} />}

      {!missing && type?.kind === "array" && (
        <ListInput
          id={field.key}
          value={Array.isArray(value) ? value.map((item) => String(item)) : []}
          onChange={(items) => {
            const parsed = items.map((item) => fromText(item, type.of)).filter((item) => item !== undefined)
            onChange(parsed as SettingsValue)
          }}
          placeholder="Type and press Enter"
        />
      )}

      {field.description && <p className="text-muted-foreground text-xs">{field.description}</p>}
    </div>
  )
}

/** number 是否按滑杆呈现: 仅当 range 恰好给出 [min, max]。 */
function isRanged(field: SettingsField): boolean {
  const { min, max } = rangeBounds(field)
  return typeof min === "number" && typeof max === "number"
}

/** 数字项: range 为 [min, max] 用滑杆; range 为空用数字输入(调用时解析文本)。 */
function NumberControl({
  field,
  value,
  onChange,
}: {
  field: SettingsField
  value: number
  onChange: (next: SettingsValue) => void
}) {
  // range 为 [min, max] 才用滑杆; range 为空则退化为手动输入(调用时解析), 不臆造边界
  const { min, max } = rangeBounds(field)
  if (!isRanged(field)) {
    return (
      <Input
        id={field.key}
        type="number"
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
        className="h-8"
      />
    )
  }
  return (
    <Slider
      id={field.key}
      min={min}
      max={max}
      value={[value]}
      onValueChange={([next]) => onChange(next)}
    />
  )
}

/**
 * 设置弹窗: 左侧分区与右侧配置项**全部由 `static/default/settings.json` 描述**,
 * 展示文案(标题/分区名/图标)取自 `sidebar.json` 的 settings 段。
 * 取值一律来自后端 settings 服务(引擎三视图叠加后的生效值), 修改经 Apply + Save 落盘; 前端不涉及默认值。
 */
export default function SettingsDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { Value, icon } = useStatic()

  const groups = React.useMemo(
    () =>
      settingsGroups().map((group) => ({
        ...group,
        Icon: icon(group.icon),
      })),
    [icon],
  )

  const [active, setActive] = React.useState("")
  const [values, setValues] = React.useState<SettingsValues>({})
  const [loaded, setLoaded] = React.useState<SettingsValues>({})
  const [pending, setPending] = React.useState(false)

  // 打开时读取生效值 (后端 List: 引擎三视图叠加结果)
  React.useEffect(() => {
    if (!open) return
    let cancelled = false
    setPending(true)
    void loadSettings()
      .then((next) => {
        if (cancelled) return
        setValues(next)
        setLoaded(next)
      })
      .catch(() => {
        if (cancelled) return
        toast.error("Failed to load settings")
      })
      .finally(() => {
        if (!cancelled) setPending(false)
      })
    return () => {
      cancelled = true
    }
  }, [open])

  // 后端事件: 落盘完成或用户文件被外部修改后重新拉取生效值
  React.useEffect(() => {
    if (!open) return
    return subscribeSettings((kind) => {
      if (kind === "changed") return
      void loadSettings()
        .then((next) => {
          setValues(next)
          setLoaded(next)
        })
        .catch(() => undefined)
    })
  }, [open])

  const activeGroup = groups.find((group) => group.id === active) ?? groups[0]
  // 分隔线只给 depth=1 的项(例 language 与 session 之间); depth>1 的项彼此不分隔, 层级由 label 里的 `>` 表达
  const groupFields = activeGroup?.fields ?? []
  const lastTopLevelIndex = groupFields.reduce(
    (last, field, index) => (field.trail.length === 0 ? index : last),
    -1,
  )

  const dirtyKeys = React.useMemo(
    () =>
      Object.keys(values).filter(
        (key) => JSON.stringify(values[key]) !== JSON.stringify(loaded[key]),
      ),
    [values, loaded],
  )

  const update = (field: SettingsField, raw: unknown) => {
    const parsed = parseFieldValue(field, raw)
    if (parsed === undefined) return
    setValues((prev) => ({ ...prev, [field.key]: parsed }))
  }

  const save = async () => {
    // 只提交改动过的键 (用户文件保持"只放覆盖项"的语义)
    const diff: SettingsValues = {}
    for (const key of dirtyKeys) diff[key] = values[key]
    try {
      await saveSettings(diff)
      setLoaded(values)
      toast.success("Settings saved")
    } catch {
      toast.error("Failed to save settings")
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="overflow-hidden p-0 md:max-h-[500px] md:max-w-[700px] lg:max-w-[800px]"
      >
        <DialogTitle className="sr-only">{Value("sidebar.settings.title")}</DialogTitle>
        <DialogDescription className="sr-only">{Value("sidebar.settings.description")}</DialogDescription>
        <SidebarProvider className="items-start">
          <Sidebar collapsible="none" className="hidden md:flex">
            <SidebarContent>
              <SidebarGroup>
                <SidebarGroupContent>
                  <SidebarMenu>
                    {groups.map((group) => (
                      <SidebarMenuItem key={group.id}>
                        <SidebarMenuButton asChild isActive={group.id === activeGroup?.id}>
                          <button type="button" onClick={() => setActive(group.id)}>
                            <group.Icon />
                            <span>{group.label}</span>
                          </button>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    ))}
                  </SidebarMenu>
                </SidebarGroupContent>
              </SidebarGroup>
            </SidebarContent>
          </Sidebar>
          <main className="flex h-[480px] flex-1 flex-col overflow-hidden">
            <header className="flex h-16 shrink-0 items-center gap-2 border-b px-4">
              <Breadcrumb>
                <BreadcrumbList>
                  <BreadcrumbItem className="hidden md:block">
                    <BreadcrumbPage>{Value("sidebar.settings.title")}</BreadcrumbPage>
                  </BreadcrumbItem>
                  <BreadcrumbSeparator className="hidden md:block" />
                  <BreadcrumbItem>
                    <BreadcrumbPage>{activeGroup?.label}</BreadcrumbPage>
                  </BreadcrumbItem>
                </BreadcrumbList>
              </Breadcrumb>
              <div className="ml-auto flex items-center gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  disabled={pending}
                  onClick={() => {
                    setValues(loaded)
                  }}
                >
                  Revert
                </Button>
                <Button
                  type="button"
                  size="sm"
                  disabled={dirtyKeys.length === 0 || pending}
                  onClick={save}
                >
                  Save
                </Button>
              </div>
            </header>
            <div className="flex-1 overflow-y-auto px-4 py-2">
              {groupFields.map((field, index) => (
                <SettingsRow
                  key={field.key}
                  field={field}
                  separator={field.trail.length === 0 && index !== lastTopLevelIndex}
                  value={values[field.key]}
                  onChange={(next) => update(field, next)}
                />
              ))}
            </div>
          </main>
        </SidebarProvider>
      </DialogContent>
    </Dialog>
  )
}
