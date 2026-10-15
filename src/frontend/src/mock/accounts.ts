/**
 * 开发期测试账号: 仅用于本地联调鉴权链路, 接入真实鉴权后删除。
 */

export type Account = {
  username: string
  password: string
}

export const accounts: Account[] = [{ username: "user1012", password: "automan" }]
