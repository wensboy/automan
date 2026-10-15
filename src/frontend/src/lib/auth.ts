import { accounts } from "@/mock/accounts"

/**
 * 会话鉴权 (当前为 mock 实现): 登录成功后在 sessionStorage 写入 token,
 * 路由守卫据此判断是否放行。接入真实鉴权时只需替换本文件。
 */

const TOKEN_KEY = "automan.token"

/** 是否已登录。 */
export function hasSession(): boolean {
  return sessionStorage.getItem(TOKEN_KEY) !== null
}

/** 校验账号并写入会话 token; 失败返回 false。 */
export function signIn(username: string, password: string): boolean {
  const matched = accounts.some(
    (account) => account.username === username && account.password === password,
  )
  if (!matched) {
    return false
  }
  sessionStorage.setItem(TOKEN_KEY, username)
  return true
}

/** 注册 (mock): 不落库, 直接写入会话 token 使注册后即可进入应用。 */
export function signUp(username: string): void {
  sessionStorage.setItem(TOKEN_KEY, username)
}

/** 清除会话 token。 */
export function signOut(): void {
  sessionStorage.removeItem(TOKEN_KEY)
}
