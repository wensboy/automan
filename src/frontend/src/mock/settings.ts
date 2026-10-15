import type { SettingsValues } from "@/static/types/settings"

/**
 * 开发期设置值 —— 替代运行时下发的生效值 (接后端后整段删除)。
 *
 * 语义说明: 正式链路由后端 settings engine 叠加三视图 (runtime > user > bottom) 后把**生效值**下发,
 * 前端不关心默认值; 这里只是"运行时的那一份数据"在 dev 阶段的占位。
 */
export const settingsRuntimeValues: SettingsValues = {
  "appearence.mode": "auto",
  "general.language": "English",
}
