/**
 * 统一的时间展示: `YYYY-MM-DD HH:mm`(可选带秒), 本地时区。
 * 日志行/表格列/侧边栏副标题都走这里, 避免各自拼一份。
 */
export function formatDateTime(ms: number, options: { seconds?: boolean } = {}): string {
  if (!Number.isFinite(ms) || ms <= 0) return ""
  const date = new Date(ms)
  const pad = (value: number) => String(value).padStart(2, "0")
  const base = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
  return options.seconds ? `${base}:${pad(date.getSeconds())}` : base
}
