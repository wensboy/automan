/**
 * Home > Environment 的开发期数据(见 docs/design/v25.md §19)。
 * 仅用于展示 file tree 结构与交互; 后续接后端时替换来源即可。
 */

export type FileNodeType = "dir" | "file"

export type FileNode = {
  id: string
  name: string
  type: FileNodeType
  children?: FileNode[]
}

export const environmentTree: FileNode[] = [
  {
    id: "dir-config",
    name: "config",
    type: "dir",
    children: [
      { id: "file-settings", name: "settings.user.json", type: "file" },
      { id: "file-about", name: "about.json", type: "file" },
      { id: "file-font", name: "font.json", type: "file" },
    ],
  },
  {
    id: "dir-data",
    name: "data",
    type: "dir",
    children: [
      { id: "file-theme", name: "theme.json", type: "file" },
      { id: "file-workspace", name: "workspace.json", type: "file" },
    ],
  },
  {
    id: "dir-logs",
    name: "logs",
    type: "dir",
    children: [
      { id: "file-app-log", name: "automan.log", type: "file" },
      { id: "file-node-log", name: "bridge-node.log", type: "file" },
    ],
  },
  { id: "file-env", name: ".env", type: "file" },
]

/** 环境变量文件(mock, 结构对齐 data/default/env.tpl.json)。 */
export const environmentFile = {
  meta: { path: "<dir>/.env.json", entry: "environments" },
  environments: [
    { type: "string", key: "API_BASE_URL", value: "http://127.0.0.1:8080" },
    { type: "int", key: "PROXY_PORT", value: "8899" },
    { type: "float", key: "TIMEOUT_SCALE", value: "1.5" },
    { type: "array", key: "ALLOWED_HOSTS", value: "[\"127.0.0.1\",\"localhost\"]" },
    { type: "object", key: "LOG_LEVELS", value: "{\"web\":\"info\",\"node\":\"warn\"}" },
  ],
}
