/**
 * 全局状态入口: 按 namespace 分文件(每个文件一个 store), 这里只做聚合导出。
 *
 * 约定:
 *   · 一个 namespace 一个文件(例 store/session.ts -> useSessionStore);
 *   · 组件内订阅用 useXxxStore(selector), 非 React 侧用对应的 xxxSnapshot();
 *   · 需要跨重启保持的状态, 在 namespace 内自行落 localStorage(见 session)。
 */
export * from "./session"
