import * as React from "react"
import { useNavigate } from "react-router"
import {
  BellIcon,
  // CreditCardIcon, // Billing 暂注释
  LogOutIcon,
  MoreVerticalIcon,
  UserCircleIcon,
} from "lucide-react"

import AccountDialog from "@/components/custom/account/account-dialog"
import NoticeDrawer from "@/components/custom/notice/notice-drawer"
import { signOut } from "@/lib/auth"
import { useSessionStore } from "@/store/session"
import { formatDateTime } from "@/lib/format"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar"
import { staticValue } from "@/components/custom/share/provider/static-provider"
import { snapshotNoticeState, subscribeNotices } from "@/lib/notice"

/**
 * 底部用户区: 头像 + 下拉菜单 (显示效果对齐 dashboard-01)。
 *
 * 注意: DropdownMenuTrigger 必须嵌在 SidebarMenuButton(asChild) 之内 ——
 * 反过来 (Trigger asChild > SidebarMenuButton) 在 React 18 下 ref 无法附着,
 * 会导致 Radix 拿不到锚点元素, 菜单弹出位置不可见 (见 v14 说明)。
 */
export default function NavUser({
  user,
}: {
  user: {
    name: string
    email: string
    avatar: string
  }
}) {
  const { isMobile } = useSidebar()
  const navigate = useNavigate()
  const me = useSessionStore((state) => state.user)
  // 标题: 昵称为空时回退登录名; 副标题: 本次会话的登录时间
  const title = me?.nickname || me?.username || user.name
  const loginAtValue = useSessionStore((state) => state.loginAt)
  const subtitle = loginAtValue ? formatDateTime(loginAtValue) : user.email
  const [noticesOpen, setNoticesOpen] = React.useState(false)
  const { unread } = React.useSyncExternalStore(subscribeNotices, snapshotNoticeState)
  const [accountOpen, setAccountOpen] = React.useState(false)

  const logout = async () => {
    await signOut()   // 通知后端结束会话并清本地状态
    navigate("/login", { replace: true })
  }

  return (
    <>
      <SidebarMenu>
        <SidebarMenuItem>
          <DropdownMenu>
            <SidebarMenuButton
              asChild
              size="lg"
              className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <DropdownMenuTrigger>
                <Avatar className="h-8 w-8 rounded-lg grayscale">
                  <AvatarImage src={user.avatar} alt={title} />
                  <AvatarFallback className="rounded-lg">CN</AvatarFallback>
                </Avatar>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-medium">{title}</span>
                  <span className="truncate text-xs text-muted-foreground">{subtitle}</span>
                </div>
                <MoreVerticalIcon className="ml-auto size-4" />
              </DropdownMenuTrigger>
            </SidebarMenuButton>
            <DropdownMenuContent
              className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
              side={isMobile ? "bottom" : "right"}
              align="end"
              sideOffset={4}
            >
              <DropdownMenuLabel className="p-0 font-normal">
                <div className="flex items-center gap-2 px-1 py-1.5 text-left text-sm">
                  <Avatar className="h-8 w-8 rounded-lg">
                    <AvatarImage src={user.avatar} alt={title} />
                    <AvatarFallback className="rounded-lg">CN</AvatarFallback>
                  </Avatar>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-medium">{title}</span>
                    <span className="truncate text-xs text-muted-foreground">{subtitle}</span>
                  </div>
                </div>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuItem onClick={() => setAccountOpen(true)}>
                  <UserCircleIcon />
                  {staticValue("sidebar.navUser.account")}
                </DropdownMenuItem>
                {/* Billing 暂注释, 后续接入 */}
                {/* <DropdownMenuItem>
                  <CreditCardIcon />
                  Billing
                </DropdownMenuItem> */}
                <DropdownMenuItem onClick={() => setNoticesOpen(true)}>
                  <BellIcon />
                  {staticValue("sidebar.navUser.notifications")}
                  {unread > 0 && (
                    <span className="ml-auto rounded-full bg-destructive px-1.5 text-[10px] font-medium leading-4 text-destructive-foreground">
                      {unread > 99 ? "99+" : unread}
                    </span>
                  )}
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={logout}>
                <LogOutIcon />
                {staticValue("sidebar.navUser.logOut")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </SidebarMenuItem>
      </SidebarMenu>
      <NoticeDrawer open={noticesOpen} onOpenChange={setNoticesOpen} />
      <AccountDialog open={accountOpen} onOpenChange={setAccountOpen} />
    </>
  )
}
