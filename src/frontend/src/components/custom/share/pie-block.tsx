import { Legend, Pie, PieChart, ResponsiveContainer, Sector, Tooltip } from "recharts"

/**
 * 环形图(基于 recharts, 但避开已弃用 API)。
 *
 * 说明(见 docs/design/v25.md §13.9):
 *   · 不用 Cell(3.x 已标弃用): 用 `shape` 渲染每个扇区, 颜色取数据的 fill;
 *   · 不用 Legend.verticalAlign(已弃用): 用 position="bottom";
 *   · 外层用 **flex 定高**(h-full + min-h-0): 由父容器决定尺寸, 随窗口缩放;
 *     不能写死像素, 也不能只给百分比高度(在 flex 居中容器里会被算成 0)。
 */
export type PieSlice = {
  name: string
  value: number
  fill: string
}

export default function PieBlock({ data }: { data: PieSlice[] }) {
  const total = data.reduce((sum, item) => sum + Number(item.value || 0), 0)
  if (total <= 0) return null

  // 用 shape 按索引取色(替代已弃用的 Cell)
  const renderSector = (props: { index?: number } & Record<string, unknown>) => {
    const fill = data[Number(props.index ?? -1)]?.fill ?? "var(--primary)"
    return <Sector {...(props as Record<string, unknown>)} fill={fill} stroke="none" />
  }

  return (
    <div className="h-full min-h-[160px] w-full">
      <ResponsiveContainer width="100%" height="100%">
        <PieChart>
          <Pie
            data={data}
            dataKey="value"
            nameKey="name"
            innerRadius="45%"
            outerRadius="72%"
            paddingAngle={2}
            strokeWidth={0}
            shape={renderSector as never}
          />
          <Tooltip
            formatter={
              ((value: number, name: string) => [
                `${value} (${Math.round((value / total) * 100)}%)`,
                name,
              ]) as never
            }
            contentStyle={{ fontSize: 12, borderRadius: 8 }}
          />
          <Legend
            position="bottom"
            iconSize={8}
            formatter={(value) => <span style={{ fontSize: 12 }}>{value}</span>}
          />
        </PieChart>
      </ResponsiveContainer>
    </div>
  )
}
