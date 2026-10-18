package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 开发期占位符: 解析为仓库根; 且目标路径必须真实存在才注入。
func TestExpandEnvDevRoot(t *testing.T) {
	root, ok := devRoot()
	if !ok {
		t.Skip("不在仓库内运行, 跳过")
	}

	// go.mod 一定存在: 既验证解析, 也验证存在性检查通过
	got := expandEnv([]string{
		"WHISTLE_BIN=${AUTOMAN_ROOT}/go.mod",
		"PLAIN=1",
	})
	if len(got) != 2 {
		t.Fatalf("应保留两条, got %v", got)
	}
	want := "WHISTLE_BIN=" + filepath.Join(root, "go.mod")
	if got[0] != want {
		t.Fatalf("开发根解析异常:\n got=%q\nwant=%q", got[0], want)
	}
	if got[1] != "PLAIN=1" {
		t.Fatalf("普通条目应原样保留, got %q", got[1])
	}

	// 目标不存在 -> 不注入(宁可没有, 也不要注入假路径)
	if dropped := expandEnv([]string{"X=${AUTOMAN_ROOT}/definitely-missing-file"}); len(dropped) != 0 {
		t.Fatalf("路径不存在时应丢弃, got %v", dropped)
	}
}

func TestExpandEnvDropsMalformedAndExpandsHome(t *testing.T) {
	// 缺 "=" / 空键 / 空条目 都不注入
	if got := expandEnv([]string{"NOEQUAL", "=x", "   "}); len(got) != 0 {
		t.Fatalf("非法条目应被丢弃, got %v", got)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("无主目录, 跳过")
	}
	got := expandEnv([]string{"CONFIG=~/.config"})
	if len(got) != 1 || got[0] != "CONFIG="+filepath.Join(home, ".config") {
		t.Fatalf("~ 展开异常, got %v", got)
	}
}

// 日志只记键名, 不泄露值。
func TestEnvKeysOnlyNames(t *testing.T) {
	if got := envKeys([]string{"WHISTLE_BIN=/secret/path", "PLAIN=1"}); got != "WHISTLE_BIN,PLAIN" {
		t.Fatalf("应只输出键名, got %q", got)
	}
	if got := envKeys(nil); got != "" {
		t.Fatalf("空输入应返回空串, got %q", got)
	}
	if strings.Contains(envKeys([]string{"A=/secret"}), "/secret") {
		t.Fatal("不应包含值")
	}
}
