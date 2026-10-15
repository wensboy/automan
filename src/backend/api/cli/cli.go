// Package cli 是命令行暴露层: 解析启动形态并驱动应用.
package cli

import (
	"io/fs"

	"github.com/wensboy/automan/backend/bootstrap"
	"github.com/wensboy/automan/backend/domain/context"
)

// Main 解析参数并运行应用.
func Main(args []string, assets, data fs.FS) error {
	return bootstrap.Run(mode(args), assets, data)
}

func mode(args []string) context.Mode {
	for _, a := range args {
		if a == "version" || a == "-v" || a == "--version" {
			return context.ModeCLI
		}
	}
	return context.ModeDesktop
}
