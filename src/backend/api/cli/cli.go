// Package cli 是程序 CLI 的唯一入口:
//   - 从 data/conf/command.json#cli 读取声明式命令树;
//   - action 的实现集中注册在这里(不散落在各处);
//   - root action 负责启动桌面(窗口); 其余命令按需执行。
package cli

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	ucli "github.com/urfave/cli/v3"

	"github.com/wensboy/automan/backend/bootstrap"
	domainconfig "github.com/wensboy/automan/backend/domain/config"
	domaincontext "github.com/wensboy/automan/backend/domain/context"
	commandcli "github.com/wensboy/automan/backend/internal/cli"
)

// configPath 是命令树的数据源(内嵌)。
//
// 说明: CLI 必须在 bootstrap(引擎装配)之前完成解析, 因此这里直接读内嵌文件;
// config 引擎在运行时才可用, 不适用于启动期。
const commandPath = "data/conf/command.json"

// Execute 解析并执行 CLI, 返回进程退出码(Unix 约定: 0 成功, 非 0 失败)。
func Execute(program domaincontext.Program) int {
	spec, err := loadCommand(program.Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "automan: %v\n", err)
		return 1
	}

	tree, err := commandcli.Build(spec, actions(spec, program))
	if err != nil {
		fmt.Fprintf(os.Stderr, "automan: %v\n", err)
		return 1
	}

	// 版本由 internal/cli.Build 挂到根命令(见 cli.Build), 这里不再重复赋值
	runErr := tree.Root.Run(stdctx.Background(), program.Args)
	if runErr == nil {
		return 0
	}
	var exitCoder ucli.ExitCoder
	if errors.As(runErr, &exitCoder) {
		if message := strings.TrimSpace(exitCoder.Error()); message != "" {
			fmt.Fprintln(os.Stderr, message)
		}
		return exitCoder.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "automan: %v\n", runErr)
	return 1
}

// actions 是 action 名 -> 实现的注册表。
// 目前只有两个: root(启动桌面) 与 version(打印版本)。
func actions(spec domainconfig.CLICommand, program domaincontext.Program) commandcli.Actions {
	return commandcli.Actions{
		"version": func(_ stdctx.Context, _ *ucli.Command) error {
			fmt.Printf("%s %s\n", spec.Name, strings.TrimSpace(spec.Version))
			return nil
		},
		// root action: 无参数时启动窗口; 带未知子命令时非 0 退出(不隐式回落)
		"root": func(_ stdctx.Context, cmd *ucli.Command) error {
			if cmd.Args().Len() > 0 {
				return ucli.Exit(fmt.Sprintf("unknown command %q", cmd.Args().First()), 1)
			}
			return bootstrap.Run(program.Assets, program.Data)
		},
	}
}

// loadCommand 读取并校验 cli 段.
func loadCommand(data fs.FS) (domainconfig.CLICommand, error) {
	if data == nil {
		return domainconfig.CLICommand{}, errors.New("cli: data source is nil")
	}
	payload, err := fs.ReadFile(data, commandPath)
	if err != nil {
		return domainconfig.CLICommand{}, fmt.Errorf("cli: read %s: %w", commandPath, err)
	}
	var spec domainconfig.Command
	if err := json.Unmarshal(payload, &spec); err != nil {
		return domainconfig.CLICommand{}, fmt.Errorf("cli: parse %s: %w", commandPath, err)
	}
	if strings.TrimSpace(spec.CLI.Name) == "" {
		return domainconfig.CLICommand{}, fmt.Errorf("cli: %s#cli.name is required", commandPath)
	}
	return spec.CLI, nil
}
