package cli

import (
	stdctx "context"
	"strings"
	"testing"

	ucli "github.com/urfave/cli/v3"

	domainconfig "github.com/wensboy/automan/backend/domain/config"
)

func actions() Actions {
	noop := func(stdctx.Context, *ucli.Command) error { return nil }
	return Actions{"root": noop, "version": noop, "proxy.list": noop}
}

// 构建: 层级/别名/action 解析正确, 索引可 O(1) 命中主名与别名路径。
func TestBuildAndLookup(t *testing.T) {
	tree, err := Build(domainconfig.CLICommand{
		Name:    "automan",
		Version: "0.0.1",
		Action:  "root",
		Commands: []domainconfig.CLICommand{
			{Name: "version", Aliases: []string{"v"}, Action: "version"},
			{Name: "proxy", Commands: []domainconfig.CLICommand{
				{Name: "list", Action: "proxy.list"},
			}},
		},
	}, actions())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, path := range []string{"version", "v", "proxy", "proxy list"} {
		if _, ok := tree.Lookup(path); !ok {
			t.Fatalf("应能查到 %q", path)
		}
	}
	if _, ok := tree.Lookup("proxy  list"); !ok { // 冗余空格归一
		t.Fatal("路径应做空白归一")
	}
	if _, ok := tree.Lookup("unknown"); ok {
		t.Fatal("不存在的路径不应命中")
	}
	if tree.Root.Action == nil || tree.Root.Version != "0.0.1" {
		t.Fatalf("root 应带 action/version: %+v", tree.Root)
	}
}

// 未注册的 action 必须在构建期报错(不静默忽略)。
func TestBuildRejectsUnknownAction(t *testing.T) {
	_, err := Build(domainconfig.CLICommand{
		Name:     "automan",
		Commands: []domainconfig.CLICommand{{Name: "version", Action: "missing"}},
	}, actions())
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("应报 action 未注册, got %v", err)
	}
}

// 深度上限与非法 name。
func TestBuildRejectsDeepOrEmpty(t *testing.T) {
	deep := domainconfig.CLICommand{Name: "a", Commands: []domainconfig.CLICommand{
		{Name: "b", Commands: []domainconfig.CLICommand{
			{Name: "c", Commands: []domainconfig.CLICommand{{Name: "d"}}},
		}},
	}}
	if _, err := Build(deep, actions()); err == nil {
		t.Fatal("超过 3 层应报错")
	}
	if _, err := Build(domainconfig.CLICommand{}, actions()); err == nil {
		t.Fatal("缺 name 应报错")
	}
}

// args / flags 的字段与会话映射(非法类型在构建期报错)。
func TestBuildFlagsAndArgs(t *testing.T) {
	tree, err := Build(domainconfig.CLICommand{
		Name:   "automan",
		Action: "root",
		Commands: []domainconfig.CLICommand{{
			Name:   "version",
			Action: "version",
			Flags: []domainconfig.CLIFlag{
				{Name: "output", Aliases: []string{"o"}, Usage: "output", Type: "string", Value: "text"},
				{Name: "verbose", Type: "bool"},
				{Name: "count", Type: "int", Value: "3"},
				{Name: "ratio", Type: "float", Value: "0.5"},
			},
			Args: []domainconfig.CLIArg{{Name: "name", Type: "string"}},
		}},
	}, actions())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	node, ok := tree.Lookup("version")
	if !ok {
		t.Fatal("应能查到 version")
	}
	if len(node.Flags) != 4 || len(node.Arguments) != 1 {
		t.Fatalf("flags/args 数量异常: %d/%d", len(node.Flags), len(node.Arguments))
	}

	if _, err := Build(domainconfig.CLICommand{
		Name: "automan", Commands: []domainconfig.CLICommand{{
			Name: "bad", Flags: []domainconfig.CLIFlag{{Name: "x", Type: "datetime"}},
		}},
	}, actions()); err == nil {
		t.Fatal("不支持的 flag 类型应报错")
	}
}
