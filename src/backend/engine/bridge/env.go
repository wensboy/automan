package bridge

import (
	"os"
	"path/filepath"
	"strings"
)

// TokenDevRoot 是只用于**开发期**的占位符: 解析为仓库根(含 go.mod 的那一层)。
// 打包运行时解析不到, 该条环境变量会被整体丢弃 —— 宁可"不注入", 也不要注入一个
// 看似有值、实际不可用的路径(那种错误更难排查)。
const TokenDevRoot = "${AUTOMAN_ROOT}"

// expandEnv 展开 env 条目里的占位符, 并丢弃无法解析的条目。
//
//	${AUTOMAN_ROOT}/x -> <仓库根>/x (仅开发期, 且要求路径真实存在)
//	~/x 或 ~          -> 用户主目录下
func expandEnv(entries []string) []string {
	expanded := make([]string, 0, len(entries))
	for _, entry := range entries {
		item, ok := expandEnvEntry(entry)
		if !ok {
			continue
		}
		expanded = append(expanded, item)
	}
	return expanded
}

func expandEnvEntry(entry string) (string, bool) {
	key, value, ok := strings.Cut(strings.TrimSpace(entry), "=")
	if !ok {
		return "", false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}

	if strings.Contains(value, TokenDevRoot) {
		root, found := devRoot()
		if !found {
			return "", false
		}
		value = strings.ReplaceAll(value, TokenDevRoot, root)
		if _, err := os.Stat(value); err != nil {
			return "", false // 路径不存在: 同样不注入
		}
	}

	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~"))
	}

	return key + "=" + value, true
}

// devRoot 判定"是否在开发期仓库内运行": 从可执行文件所在目录与当前工作目录分别向上找 go.mod。
// 打包后在用户机器上两者都不存在 go.mod, 因此不会命中。
func devRoot() (string, bool) {
	starts := make([]string, 0, 2)
	if executable, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(executable))
	}
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}

	for _, start := range starts {
		for dir := start; ; {
			if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
				return dir, true
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", false
}

// envKeys 取环境变量条目的键名(只用于日志, 避免把值写进日志)。
func envKeys(entries []string) string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		if key, _, ok := strings.Cut(entry, "="); ok {
			keys = append(keys, strings.TrimSpace(key))
		}
	}
	return strings.Join(keys, ",")
}
