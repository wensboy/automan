import { create } from "zustand"

import {
  Create,
  Current,
  Delete,
  List,
  SetCurrent,
} from "../../bindings/github.com/wensboy/automan/backend/api/web/workspaceservice"
import type { Workspace } from "../../bindings/github.com/wensboy/automan/backend/service/workspace/models"

/**
 * 工作区状态 (namespace: workspace)。
 *
 * 定位: 工作区列表 + 当前工作区是"挂载卸载后无法重新推导"的唯一信息, 因此入 store;
 *       后端仍是权威 —— 每次变更都先调服务, 再用返回值刷新本地。
 */
export type WorkspaceState = {
  items: Workspace[]
  currentId: string
  /** 每次都实时向后端取(不做缓存, 避免跨用户/跨工作区残留) */
  load: () => Promise<void>
  create: (name: string) => Promise<void>
  remove: (id: string) => Promise<void>
  select: (id: string) => Promise<void>
  /** 清空本地副本(用户切换/登出时调用) */
  reset: () => void
}

export const useWorkspaceStore = create<WorkspaceState>((set, get) => ({
  items: [],
  currentId: "",

  load: async () => {
    const [items, currentId] = await Promise.all([List(), Current()])
    set({ items: items ?? [], currentId: currentId ?? "" })
  },

  create: async (name) => {
    const created = await Create(name)
    if (created?.id) {
      await SetCurrent(created.id)
    }
    await get().load()
  },

  remove: async (id) => {
    await Delete(id)
    const current = get().currentId
    if (current === id) {
      // 后端会清空当前; 重新拉取时 List 会自动补一个 Default
      set({ currentId: "" })
    }
    await get().load()
  },

  select: async (id) => {
    await SetCurrent(id)
    set({ currentId: id })
  },

  reset: () => set({ items: [], currentId: "" }),
}))

/** 非 React 上下文读取当前工作区 id。 */
export function currentWorkspaceID(): string {
  return useWorkspaceStore.getState().currentId
}
