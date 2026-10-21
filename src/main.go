package main

import (
	"embed"
	"os"

	"github.com/wensboy/automan/backend/api/cli"
	domaincontext "github.com/wensboy/automan/backend/domain/context"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed data
var data embed.FS

// main 只做一件事: 把外部输入交给 CLI, 并以其退出码结束进程.
// 桌面启动、子命令分发都由 command.json#cli 的命令树与 api/cli 的 action 决定。
func main() {
	os.Exit(cli.Execute(domaincontext.Program{
		Args:   os.Args,
		Assets: assets,
		Data:   data,
	}))
}
