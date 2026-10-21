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

// Command 映射 data/conf/command.json: cli 段是程序 CLI 解析配置, runtime 段供前端命令面板.
type Command struct {
	CLI     map[string]any `json:"cli"`
	Runtime CommandRuntime `json:"runtime"`
}

// CommandKey 取 conf/command 命名空间根.
var CommandKey = key.New[Command]("conf/command")
