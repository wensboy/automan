import { Events } from "@wailsio/runtime"

import {
  Resume,
  SignIn,
  SignOut,
  SignUp,
} from "../../bindings/github.com/wensboy/automan/backend/api/web/sessionservice"
import type { Session, SignUpInput } from "../../bindings/github.com/wensboy/automan/backend/service/session"
import type { User } from "../../bindings/github.com/wensboy/automan/backend/service/user"

/**
 * 会话鉴权 (前端唯一出口, 对应 v18):
 * - token 存 localStorage: 跨重启保持; 后端按 login_logs 里"最后登录时间 + 有效期"判定是否仍有效;
 * - 所有动作都经 api/web 的 SessionService, 前端不参与口令/会话逻辑, 只保存结果;
 * - 后端事件 session:logout / session:expired 到达时清掉本地会话。
 */

const TOKEN_KEY = "automan.token"
const USER_KEY = "automan.user"
const LOGIN_AT_KEY = "automan.loginAt"

/** 当前会话 token (未登录返回 null)。 */
export function storedToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

/** 当前登录用户 (仅供展示; 权威判断始终在后端)。 */
export function currentUser(): User | null {
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as User
  } catch {
    return null
  }
}

/** 本次会话的登录时刻 (毫秒; 未登录返回 null)。 */
export function loginAt(): number | null {
  const raw = localStorage.getItem(LOGIN_AT_KEY)
  if (!raw) return null
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

/** 清掉本地会话 (不调用后端)。 */
export function clearSession(): void {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  localStorage.removeItem(LOGIN_AT_KEY)
}

function persist(session: Session): User {
  localStorage.setItem(TOKEN_KEY, session.token)
  localStorage.setItem(USER_KEY, JSON.stringify(session.user))
  localStorage.setItem(LOGIN_AT_KEY, String(session.loginAt))
  return session.user
}

/** 资料更新后回写本地用户(仅展示用; 权威在后端)。 */
export function updateStoredUser(user: User): void {
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}


/** 登录; 失败抛出后端给出的原因(用户不存在 / 口令错误等)。 */
export async function signIn(username: string, password: string): Promise<User> {
  return persist(await SignIn(username, password))
}

/** 注册并立即登录。 */
export async function signUp(input: SignUpInput): Promise<User> {
  return persist(await SignUp(input))
}

/** 登出: 通知后端结束会话, 并清掉本地状态(后端失败也清, 避免卡在登录态)。 */
export async function signOut(): Promise<void> {
  const token = storedToken()
  clearSession()
  if (!token) return
  try {
    await SignOut(token)
  } catch {
    // 会话可能已过期, 本地已清理, 不再上抛
  }
}

/** 启动恢复: 用本地 token 调 Resume(后端按登录日志 + 有效期判定), 失败即视为未登录。 */
export async function restore(): Promise<User | null> {
  const token = storedToken()
  if (!token) return null
  try {
    return persist(await Resume(token))
  } catch {
    clearSession()
    return null
  }
}

/** 订阅后端会话事件; logout/expired 会先清本地会话再回调。返回退订函数。 */
export function onSessionEvent(
  handler: (kind: "login" | "logout" | "signup" | "expired") => void,
): () => void {
  const offs = [
    Events.On("session:login", () => handler("login")),
    Events.On("session:signup", () => handler("signup")),
    Events.On("session:logout", () => {
      clearSession()
      handler("logout")
    }),
    Events.On("session:expired", () => {
      clearSession()
      handler("expired")
    }),
  ]
  return () => {
    for (const off of offs) off()
  }
}
