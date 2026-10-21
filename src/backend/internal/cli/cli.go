// Package cli 把 data/conf/command.json#cli 的声明式命令树转成 urfave/cli 的 *cli.Command,
// 并建立 "命令路径 -> 节点" 的索引(运行期 O(1) 查找)。
//
// 约定:
//   - action 在配置里只是名字, 真实实现由调用方通过 Actions 注册表提供; 缺注册 => 构建期报错;
//   - 命令树深度上限 3(防止配置写出无限层级);
//   - 索引包含主名与别名路径(空格分隔, 如 "proxy list")。
package cli

import (
	"fmt"
	"strconv"
	"strings"

	ucli "github.com/urfave/cli/v3"

	domainconfig "github.com/wensboy/automan/backend/domain/config"
)

// Actions 是 action 名到实现的注册表(空串代表 root action).
type Actions map[string]ucli.ActionFunc

// Tree 是构建结果: 根命令 + 路径索引.
type Tree struct {
	Root  *ucli.Command
	index map[string]*ucli.Command
}

// maxDepth 是命令树深度上限(根 = 1).
const maxDepth = 3

// Build 把配置树转成 urfave/cli 命令树.
func Build(cfg domainconfig.CLICommand, actions Actions) (*Tree, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, fmt.Errorf("cli: root command name is required")
	}
	index := make(map[string]*ucli.Command)
	root, err := buildNode(cfg, actions, "", index, 1)
	if err != nil {
		return nil, err
	}
	return &Tree{Root: root, index: index}, nil
}

// Lookup 按路径查找命令节点(例 "version" / "proxy list"); 未命中返回 false.
func (t *Tree) Lookup(path string) (*ucli.Command, bool) {
	if t == nil {
		return nil, false
	}
	node, ok := t.index[normalizePath(path)]
	return node, ok
}

func buildNode(
	cfg domainconfig.CLICommand,
	actions Actions,
	parentPath string,
	index map[string]*ucli.Command,
	depth int,
) (*ucli.Command, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("cli: command tree exceeds %d levels at %q", maxDepth, cfg.Name)
	}
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return nil, fmt.Errorf("cli: command name is required (parent %q)", parentPath)
	}

	flags, err := buildFlags(name, cfg.Flags)
	if err != nil {
		return nil, err
	}
	args, err := buildArgs(name, cfg.Args)
	if err != nil {
		return nil, err
	}

	node := &ucli.Command{
		Name:        name,
		Aliases:     cfg.Aliases,
		Usage:       cfg.Usage,
		Description: cfg.Description,
		Hidden:      cfg.Hidden,
		Flags:       flags,
		Arguments:   args,
	}

	// 根命令(depth=1)的路径记为空串 -> 子命令路径是 "version" / "proxy list"(不含程序名)
	path := joinPath(parentPath, name)
	if depth == 1 {
		path = ""
		node.Version = strings.TrimSpace(cfg.Version) // 版本只挂根命令(-v/--version)
	}
	if _, exists := index[path]; exists {
		return nil, fmt.Errorf("cli: duplicate command path %q", path)
	}
	index[path] = node
	for _, alias := range cfg.Aliases {
		trimmed := strings.TrimSpace(alias)
		if trimmed == "" {
			continue
		}
		if depth == 1 {
			continue // 根命令的别名不进路径索引(根路径固定为空串)
		}
		index[joinPath(parentPath, trimmed)] = node
	}

	if action := strings.TrimSpace(cfg.Action); action != "" {
		handler, ok := actions[action]
		if !ok {
			return nil, fmt.Errorf("cli: action %q of command %q is not registered", action, path)
		}
		node.Action = handler
	}

	for _, child := range cfg.Commands {
		built, childErr := buildNode(child, actions, path, index, depth+1)
		if childErr != nil {
			return nil, childErr
		}
		node.Commands = append(node.Commands, built)
	}
	return node, nil
}

func buildFlags(command string, specs []domainconfig.CLIFlag) ([]ucli.Flag, error) {
	flags := make([]ucli.Flag, 0, len(specs))
	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			return nil, fmt.Errorf("cli: flag name is required (command %q)", command)
		}
		switch strings.ToLower(strings.TrimSpace(spec.Type)) {
		case "", "string":
			flags = append(flags, &ucli.StringFlag{
				Name: name, Aliases: spec.Aliases, Usage: spec.Usage,
				Value: spec.Value, Required: spec.Required,
			})
		case "bool":
			value := false
			if trimmed := strings.TrimSpace(spec.Value); trimmed != "" {
				parsed, err := strconv.ParseBool(trimmed)
				if err != nil {
					return nil, fmt.Errorf("cli: flag %q default %q is not bool", name, spec.Value)
				}
				value = parsed
			}
			flags = append(flags, &ucli.BoolFlag{
				Name: name, Aliases: spec.Aliases, Usage: spec.Usage,
				Value: value, Required: spec.Required,
			})
		case "int":
			value, err := parseIntDefault(name, spec.Value)
			if err != nil {
				return nil, err
			}
			flags = append(flags, &ucli.IntFlag{
				Name: name, Aliases: spec.Aliases, Usage: spec.Usage,
				Value: value, Required: spec.Required,
			})
		case "float", "float64":
			value, err := parseFloatDefault(name, spec.Value)
			if err != nil {
				return nil, err
			}
			flags = append(flags, &ucli.FloatFlag{
				Name: name, Aliases: spec.Aliases, Usage: spec.Usage,
				Value: value, Required: spec.Required,
			})
		default:
			return nil, fmt.Errorf("cli: flag %q has unsupported type %q", name, spec.Type)
		}
	}
	return flags, nil
}

func buildArgs(command string, specs []domainconfig.CLIArg) ([]ucli.Argument, error) {
	args := make([]ucli.Argument, 0, len(specs))
	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			return nil, fmt.Errorf("cli: argument name is required (command %q)", command)
		}
		switch strings.ToLower(strings.TrimSpace(spec.Type)) {
		case "", "string":
			args = append(args, &ucli.StringArg{Name: name, UsageText: spec.Usage, Value: spec.Value})
		case "int":
			value, err := parseIntDefault(name, spec.Value)
			if err != nil {
				return nil, err
			}
			args = append(args, &ucli.IntArg{Name: name, UsageText: spec.Usage, Value: value})
		case "float", "float64":
			value, err := parseFloatDefault(name, spec.Value)
			if err != nil {
				return nil, err
			}
			args = append(args, &ucli.FloatArg{Name: name, UsageText: spec.Usage, Value: value})
		default:
			return nil, fmt.Errorf("cli: argument %q has unsupported type %q", name, spec.Type)
		}
	}
	return args, nil
}

func parseIntDefault(name, raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("cli: %q default %q is not int", name, raw)
	}
	return value, nil
}

func parseFloatDefault(name, raw string) (float64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("cli: %q default %q is not float", name, raw)
	}
	return value, nil
}

func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + " " + name
}

func normalizePath(path string) string {
	return strings.Join(strings.Fields(path), " ")
}
