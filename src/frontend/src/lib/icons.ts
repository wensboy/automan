import type { LucideIcon } from "lucide-react"
import {
  SaveIcon,
  Trash2Icon,
  PencilIcon,
  ListIcon,
  FolderPlusIcon,
  FolderOpenIcon,
  FolderIcon,
  FilePlusIcon,
  FileIcon,
  CopyIcon,
  AudioWaveform,
  BellIcon,
  ChartColumnIcon,
  CheckIcon,
  ClipboardListIcon,
  CoffeeIcon,
  Command,
  DatabaseIcon,
  GalleryVerticalEnd,
  GlobeIcon,
  HelpCircleIcon,
  HouseIcon,
  InfoIcon,
  KeyboardIcon,
  LayoutTemplateIcon,
  LinkIcon,
  LockIcon,
  MenuIcon,
  MessageCircleIcon,
  PaletteIcon,
  RadarIcon,
  RefreshCwIcon,
  RouteIcon,
  ScrollText,
  SearchIcon,
  ServerIcon,
  SettingsIcon,
  ShieldCheckIcon,
  VideoIcon,
  WorkflowIcon,
  WrenchIcon,
  ZapIcon,
} from "lucide-react"

/**
 * 静态数据里的图标名(kebab-case) -> lucide 组件。
 *
 * 唯一映射表: sidebar / about 等所有静态结构共用, 新增图标只在这里加一行。
 * 未登记名称视为数据错误, 直接抛错(构建期数据, 不静默兜底)。
 */
const iconRegistry: Record<string, LucideIcon> = {
  "audio-waveform": AudioWaveform,
  "bell": BellIcon,
  "chart-column": ChartColumnIcon,
  "check": CheckIcon,
  "clipboard-list": ClipboardListIcon,
  "command": Command,
  "database": DatabaseIcon,
  "gallery-vertical-end": GalleryVerticalEnd,
  "globe": GlobeIcon,
  "help-circle": HelpCircleIcon,
  "house": HouseIcon,
  "info": InfoIcon,
  "keyboard": KeyboardIcon,
  "layout-template": LayoutTemplateIcon,
  "link": LinkIcon,
  "lock": LockIcon,
  "menu": MenuIcon,
  "message-circle": MessageCircleIcon,
  "palette": PaletteIcon,
  "radar": RadarIcon,
  "route": RouteIcon,
  "scroll-text": ScrollText,
  "search": SearchIcon,
  "server": ServerIcon,
  "settings": SettingsIcon,
  "shield-check": ShieldCheckIcon,
  "video": VideoIcon,
  "workflow": WorkflowIcon,
  "wrench": WrenchIcon,
  "zap": ZapIcon,
  "file": FileIcon,
  "file-plus": FilePlusIcon,
  "folder": FolderIcon,
  "folder-open": FolderOpenIcon,
  "folder-plus": FolderPlusIcon,
  "list": ListIcon,
  "pencil": PencilIcon,
  "copy": CopyIcon,
  "trash-2": Trash2Icon,
  "save": SaveIcon,
  "coffee": CoffeeIcon,
  "refresh-cw": RefreshCwIcon,
}

export function resolveIcon(name: string): LucideIcon {
  const icon = iconRegistry[name]
  if (!icon) {
    throw new Error(`静态数据: 未登记的图标 "${name}"`)
  }
  return icon
}
