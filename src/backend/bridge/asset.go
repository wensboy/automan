// Package bridge 提供被桥接进程 (backend/bridge/node) 构建产物的内嵌资源。
// 产物不入库, 由构建流程生成 (pnpm run build); 这里只负责把它链进 Go 二进制,
// 供桥接引擎在启动时落到运行目录再拉起。
package bridge

import (
	"embed"
	"io/fs"
)

//go:embed all:node/dist
var assets embed.FS

// Asset 返回 node 侧产物 (根目录下为 index.js)。
func Asset(path string) fs.FS {
	sub, err := fs.Sub(assets, path)
	if err != nil {
		// 路径是常量, 失败说明内嵌内容异常; 交给调用方的读取报错暴露。
		return assets
	}
	return sub
}
