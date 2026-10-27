import { Info } from "../../bindings/github.com/wensboy/automan/backend/api/web/aboutservice"

type InfoModel = Awaited<ReturnType<typeof Info>>
import { staticStore } from "@/lib/static-data"

/**
 * Home > About 的数据/显示映射层(见 docs/design/v25.md §17)。
 *
 * 约定:
 *   · **后端是唯一数据来源**(conf/about + 构建注入), 前端不做任何兜底计算;
 *   · 这里只负责: 取数 -> label 映射(contentArea.home.about.properties.label)
 *     -> 展示格式化(时间/去协议/拼链接) -> 交给组件直接渲染。
 */

export type AboutRow = {
  key: string
  label: string
  value: string
  /** 可点击时给出外链 */
  href?: string
}

export type AboutAction = {
  id: string
  icon: string
  label: string
}

export type AboutModel = {
  appName: string
  description: string
  rows: AboutRow[]
  actions: AboutAction[]
  /** 原始仓库地址(用于派生外部链接); 为空时不跳转 */
  repositoryUrl: string
}

/** 读取 footer 动作(icon+label), 结构统一为 [{id,icon,label}]。 */
function loadActions(): AboutAction[] {
  const items = staticStore.view().Value("contentArea.home.about.actions")
  if (!Array.isArray(items)) return []
  return items.map((item) => ({
    id: String((item as Record<string, unknown>).id ?? ""),
    icon: String((item as Record<string, unknown>).icon ?? ""),
    label: String((item as Record<string, unknown>).label ?? ""),
  }))
}

function labelOf(key: string): string {
  const value = staticStore.view().Value(`contentArea.home.about.properties.label.${key}`)
  return typeof value === "string" ? value : key
}


/** ISO/RFC3339 -> `YYYY-MM-DD HH:mm:ss`(本地时区); 解析失败原样返回。 */
function formatBuildTime(value: string): string {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (input: number) => String(input).padStart(2, "0")
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

/** 去掉协议前缀用于展示(链接仍用原值)。 */
function withoutProtocol(url: string): string {
  return url.replace(/^https?:\/\//, "")
}

/** 把后端属性映射成可直接渲染的行(顺序固定, 与 contentArea label 对应)。 */
function toRows(info: InfoModel): AboutRow[] {
  const properties = info?.properties
  const repository = properties?.repository ?? ""
  const buildHash = properties?.buildHash ?? ""

  return [
    { key: "version", label: labelOf("version"), value: properties?.version || "-" },
    { key: "buildTime", label: labelOf("buildTime"), value: formatBuildTime(properties?.buildTime ?? "") },
    {
      key: "buildHash",
      label: labelOf("buildHash"),
      value: buildHash || "-",
      href: repository && buildHash ? `${repository}/commit/${buildHash}` : undefined,
    },
    {
      key: "license",
      label: labelOf("license"),
      value: properties?.license || "-",
      href: properties?.licenseUrl || undefined,
    },
    {
      key: "repository",
      label: labelOf("repository"),
      value: repository ? withoutProtocol(repository) : "-",
      href: repository || undefined,
    },
  ]
}

/** 拉取 about 信息并完成显示映射。 */
export async function loadAbout(): Promise<AboutModel> {
  const info = await Info()
  return {
    appName: info?.appName || "-",
    description: info?.description ?? "",
    rows: toRows(info),
    actions: loadActions(),
    repositoryUrl: info?.properties?.repository ?? "",
  }
}

/** Check update / Buy a coffee 的跳转地址(由仓库地址派生)。 */
export function aboutLinks(repositoryUrl: string): { releases: string; sponsor: string } {
  const base = repositoryUrl || ""
  return {
    releases: base ? `${base}/releases` : "",
    sponsor: "https://github.com/sponsors/wensboy",
  }
}
