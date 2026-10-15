package settings

import (
	"sort"
	"testing"
)

// 落盘键过滤: 用户文件只承载允许的键, 程序参数/运行态键不写进去(见 v15 第 12 节 B).
func TestFilterOverrides(t *testing.T) {
	overrides := map[string]any{
		"appearence.mode":  "dark",
		"general.language": "Chinese",
		"lastGreet":        "wens",
	}
	allow := func(name string) bool { return name != "lastGreet" }

	kept := filterOverrides(overrides, allow)
	names := make([]string, 0, len(kept))
	for name := range kept {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "appearence.mode" || names[1] != "general.language" {
		t.Fatalf("过滤结果异常: %v", names)
	}
	if len(overrides) != 3 {
		t.Fatalf("不应改动入参")
	}
}

// allow 为空表示不过滤(引擎默认行为保持不变).
func TestFilterOverridesNoFilter(t *testing.T) {
	overrides := map[string]any{"a": 1}
	if got := filterOverrides(overrides, nil); len(got) != 1 {
		t.Fatalf("空过滤器应原样返回, got %v", got)
	}
}
