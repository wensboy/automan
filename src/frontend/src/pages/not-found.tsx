import { Link } from "react-router"


import EmptyState from "@/components/custom/share/empty-state"
import { Button } from "@/components/ui/button"
import { useStatic } from "@/components/custom/share/provider/static-provider"

/** 未匹配路由时的兜底页面 (与其他空状态同一表现, 纯英文)。 */
export default function NotFound() {
  const { Value } = useStatic()

  return (
    <EmptyState
      title={Value("contentArea.notFound.title") ?? ""}
      description={Value("contentArea.notFound.description")}
    >
      <Button asChild size="sm">
        <Link to="/">{Value("contentArea.notFound.back")}</Link>
      </Button>
    </EmptyState>
  )
}
