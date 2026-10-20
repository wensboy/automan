import { create } from "zustand"

import type { User } from "../../bindings/github.com/wensboy/automan/backend/service/user"

/**
 * 会话状态 (namespace: session)。
 *
 * 定位: 前端**唯一**的会话状态源。组件通过 useSessionStore 订阅, 非 React 侧用 sessionSnapshot()。
 * 持久化: 仍写既有的三个 localStorage 键(automan.token / automan.user / automan.loginAt),
 *         保持与旧版本/后端"按登录日志恢复"的语义兼容; 后端始终是权威, 这里只是展示与路由判断用。
 */

const TOKEN_KEY = "automan.token"
const USER_KEY = "automan.user"
const LOGIN_AT_KEY = "automan.loginAt"

export type SessionSnapshot = {
  token: string | null
  user: User | null
  loginAt: number | null
}

export type SessionState = SessionSnapshot & {
  /** 登录/注册/恢复成功后写入(同时落 localStorage)。 */
  setSession: (session: { token: string; user: User; loginAt: number }) => void
  /** 资料更新后回写用户(仅展示用)。 */
  patchUser: (user: User) => void
  /** 清空本地会话(不调用后端)。 */
  clearSession: () => void
}

function readToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

function readUser(): User | null {
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as User
  } catch {
    return null
  }
}

function readLoginAt(): number | null {
  const raw = localStorage.getItem(LOGIN_AT_KEY)
  if (!raw) return null
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

export const useSessionStore = create<SessionState>((set) => ({
  token: readToken(),
  user: readUser(),
  loginAt: readLoginAt(),

  setSession: ({ token, user, loginAt }) => {
    localStorage.setItem(TOKEN_KEY, token)
    localStorage.setItem(USER_KEY, JSON.stringify(user))
    localStorage.setItem(LOGIN_AT_KEY, String(loginAt))
    set({ token, user, loginAt })
  },

  patchUser: (user) => {
    localStorage.setItem(USER_KEY, JSON.stringify(user))
    set({ user })
  },

  clearSession: () => {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    localStorage.removeItem(LOGIN_AT_KEY)
    set({ token: null, user: null, loginAt: null })
  },
}))

/** 非 React 上下文(路由判断/工具函数)读取当前快照。 */
export function sessionSnapshot(): SessionSnapshot {
  const { token, user, loginAt } = useSessionStore.getState()
  return { token, user, loginAt }
}
