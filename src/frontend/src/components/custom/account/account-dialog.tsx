import * as React from "react"
import { PencilIcon, RefreshCwIcon, SaveIcon } from "lucide-react"
import { toast } from "sonner"

import LabelInput from "@/components/custom/share/label-input"

import SwitchButton from "@/components/custom/share/switch-button"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useStatic, staticValue } from "@/components/custom/share/provider/static-provider"

/** 卡片操作行: 从右到左 [编辑(开关) / 保存 / 刷新]; dev 阶段只做反馈, 后续替换为实际行为。 */
function CardActions({
  editing,
  onToggleEdit,
  onSave,
  onRefresh,
}: {
  editing: boolean
  onToggleEdit: () => void
  onSave: () => void
  onRefresh: () => void
}) {
  return (
    <div className="flex items-center gap-1">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7"
        aria-label={staticValue("sidebar.account.refresh")}
        onClick={onRefresh}
      >
        <RefreshCwIcon className="size-3.5" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7"
        aria-label={staticValue("sidebar.account.save")}
        onClick={onSave}
      >
        <SaveIcon className="size-3.5" />
      </Button>
      <SwitchButton
        active={editing}
        onToggle={onToggleEdit}
        className="size-7"
        aria-label={staticValue("sidebar.account.edit")}
      >
        <PencilIcon className="size-3.5" />
      </SwitchButton>
    </div>
  )
}

/** 账户弹窗: account / password 两张卡片, 各自独立编辑态。 */
export default function AccountDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { sidebar } = useStatic()

  const [name, setName] = React.useState(sidebar.user.name)
  const [email, setEmail] = React.useState(sidebar.user.email)
  const [avatar, setAvatar] = React.useState("/placeholder.svg")
  const [accountEditing, setAccountEditing] = React.useState(false)
  const [passwordEditing, setPasswordEditing] = React.useState(false)
  const [currentPassword, setCurrentPassword] = React.useState("")
  const [newPassword, setNewPassword] = React.useState("")
  const [confirmPassword, setConfirmPassword] = React.useState("")
  const fileRef = React.useRef<HTMLInputElement>(null)

  const pickAvatar = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return
    setAvatar((previous) => {
      if (previous.startsWith("blob:")) {
        URL.revokeObjectURL(previous)
      }
      return URL.createObjectURL(file)
    })
    toast.success(staticValue("sidebar.account.avatarUpdated") ?? "")
  }

  const saveAccount = () => {
    setAccountEditing(false)
    toast.success(staticValue("sidebar.account.informationSaved") ?? "")
  }

  const savePassword = () => {
    setPasswordEditing(false)
    toast.success(staticValue("sidebar.account.passwordSaved") ?? "")
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false} className="sm:max-w-lg" aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle>{staticValue("sidebar.account.title")}</DialogTitle>
        </DialogHeader>

        <Card>
          <CardHeader>
            <CardTitle>{staticValue("sidebar.account.information")}</CardTitle>
            <CardAction>
              <CardActions
                editing={accountEditing}
                onToggleEdit={() => setAccountEditing((prev) => !prev)}
                onSave={saveAccount}
                onRefresh={() => toast.success(staticValue("sidebar.account.informationRefreshed") ?? "")}
              />
            </CardAction>
          </CardHeader>
          <CardContent className="flex items-start gap-6">
            <div className="flex flex-1 flex-col gap-3">
              <LabelInput
                label={staticValue("sidebar.account.name") ?? ""}
                value={name}
                readOnly={!accountEditing}
                onChange={(event) => setName(event.target.value)}
              />
              <LabelInput
                label={staticValue("sidebar.account.email") ?? ""}
                value={email}
                readOnly={!accountEditing}
                onChange={(event) => setEmail(event.target.value)}
              />
            </div>
            <Avatar
              className="size-20 shrink-0 cursor-pointer rounded-full"
              onClick={() => fileRef.current?.click()}
            >
              <AvatarImage src={avatar} alt={staticValue("sidebar.account.avatarAlt") ?? ""} />
              <AvatarFallback>CN</AvatarFallback>
            </Avatar>
            <input
              ref={fileRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={pickAvatar}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{staticValue("sidebar.account.password")}</CardTitle>
            <CardAction>
              <CardActions
                editing={passwordEditing}
                onToggleEdit={() => setPasswordEditing((prev) => !prev)}
                onSave={savePassword}
                onRefresh={() => toast.success(staticValue("sidebar.account.passwordRefreshed") ?? "")}
              />
            </CardAction>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <LabelInput
              label={staticValue("sidebar.account.currentPassword") ?? ""}
              type="password"
              value={currentPassword}
              readOnly={!passwordEditing}
              onChange={(event) => setCurrentPassword(event.target.value)}
            />
            <LabelInput
              label={staticValue("sidebar.account.newPassword") ?? ""}
              type="password"
              value={newPassword}
              readOnly={!passwordEditing}
              onChange={(event) => setNewPassword(event.target.value)}
            />
            <LabelInput
              label={staticValue("sidebar.account.confirmPassword") ?? ""}
              type="password"
              value={confirmPassword}
              readOnly={!passwordEditing}
              onChange={(event) => setConfirmPassword(event.target.value)}
            />
          </CardContent>
        </Card>
      </DialogContent>
    </Dialog>
  )
}
