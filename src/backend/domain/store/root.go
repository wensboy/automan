package store

// Rooter 暴露存储根 (绝对路径), 供需要真实文件路径的引擎使用 (如 uds).
// 目的: 统一根的唯一来源, 避免各引擎重复实现路径规则.
type Rooter interface {
	Root() string
}
