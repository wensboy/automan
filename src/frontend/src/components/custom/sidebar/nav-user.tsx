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
  const [noticesOpen, setNoticesOpen] = React.useState(false)
  const [accountOpen, setAccountOpen] = React.useState(false)

  const logout = () => {
    signOut()
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
                  <AvatarImage src={user.avatar} alt={user.name} />
                  <AvatarFallback className="rounded-lg">CN</AvatarFallback>
                </Avatar>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-medium">{user.name}</span>
                  <span className="truncate text-xs text-muted-foreground">{user.email}</span>
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
                    <AvatarImage src={user.avatar} alt={user.name} />
                    <AvatarFallback className="rounded-lg">CN</AvatarFallback>
                  </Avatar>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-medium">{user.name}</span>
                    <span className="truncate text-xs text-muted-foreground">{user.email}</span>
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
