package config

import (
	"encoding/json"

	"github.com/wensboy/automan/backend/domain/key"
)

// CommandEntry 是命令清单里的一条(字段与 command.json 对应).
type CommandEntry struct {
	Cmd   string `json:"cmd"`
	Label string `json:"label"`
	KBD   string `json:"kbd"`
}

// CommandRuntime 是 runtime 段: groups 给顺序, 其余键按组名动态承载.
//
// command.json 的分组是**动态键**(setting/session/proxy/automation...),
// 因此实现 UnmarshalJSON 把非 groups 的键收进 Items。
type CommandRuntime struct {
	Groups []string
	Items  map[string][]CommandEntry
}

// UnmarshalJSON 解析 runtime 段(groups 定顺序, 其它键视为分组条目).
func (r *CommandRuntime) UnmarshalJSON(data []byte) error {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Items = map[string][]CommandEntry{}
	if groupsRaw, ok := raw["groups"]; ok {
		if err := json.Unmarshal(groupsRaw, &r.Groups); err != nil {
			return err
		}
	}
	for name, entriesRaw := range raw {
		if name == "groups" {
			continue
		}
		var entries []CommandEntry
		if err := json.Unmarshal(entriesRaw, &entries); err != nil {
			continue // 非数组的键忽略(例如未来的扩展位)
		}
		r.Items[name] = entries
	}
	return nil
}

// CLICommand 是 cli 段的递归结构(与 urfave/cli 的 Command 对应, 只取最小字段集合).
//
// action 在 JSON 里只写**名字**: 真实实现必须存在于 api/cli 的注册表, 否则启动期报错.
type CLICommand struct {
	Name        string       `json:"name"`
	Aliases     []string     `json:"aliases"`
	Usage       string       `json:"usage"`
	Version     string       `json:"version"`
	Description string       `json:"description"`
	Hidden      bool         `json:"hidden"`
	Args        []CLIArg     `json:"args"`
	Flags       []CLIFlag    `json:"flags"`
	Action      string       `json:"action"`
	Commands    []CLICommand `json:"commands"` // 递归
}

// CLIArg 是一个位置参数(name/usage/type/required).
type CLIArg struct {
	Name     string `json:"name"`
	Usage    string `json:"usage"`
	Type     string `json:"type"` // string(默认) / int / float
	Required bool   `json:"required"`
	Value    string `json:"value"`
}

// CLIFlag 是一个开关(name/aliases/usage/type/default/required).
type CLIFlag struct {
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases"`
	Usage    string   `json:"usage"`
	Type     string   `json:"type"` // string(默认) / bool / int / float
	Value    string   `json:"value"`
	Required bool     `json:"required"`
}

// Command 映射 data/conf/command.json: cli 段是程序 CLI 的命令树, runtime 段供前端命令面板.
type Command struct {
	CLI     CLICommand     `json:"cli"`
	Runtime CommandRuntime `json:"runtime"`
}

// CommandKey 取 conf/command 命名空间根.
var CommandKey = key.New[Command]("conf/command")
