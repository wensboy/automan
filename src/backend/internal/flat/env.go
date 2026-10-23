// Package flat 提供轻量的"模板展开"工具: 用给定模板替换输入串中的占位符。
//
// 之所以单独成包(见 docs/design/v23.md §37): 不只是 env 需要展开 ——
// 启动参数、路径、sock 名称都可能需要"模板 + 输入 -> 输出"的同一种处理;
// 这里只做纯粹的字符串替换与批量过滤, 不感知具体业务(不知道 AUTOMAN_ROOT 是什么)。
package flat

import "strings"

// Resolve 用 resolve 产出的值替换 input 中的 pattern。
//
//   - pattern 为占位符原文(例 "${AUTOMAN_ROOT}");
//   - resolve 返回 (替换值, 是否可用); 不可用表示**整条输入应被丢弃**(返回 false);
//   - 返回 (替换结果, 是否保留)。
func Resolve(input, pattern string, resolve func() (string, bool)) (string, bool) {
	if !strings.Contains(input, pattern) {
		return input, true
	}
	value, ok := resolve()
	if !ok || value == "" {
		return "", false
	}
	return strings.ReplaceAll(input, pattern, value), true
}

// ResolveAll 对一组输入做同样的展开, 丢弃不可解析的条目。
func ResolveAll(entries []string, pattern string, resolve func() (string, bool)) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		if resolved, ok := Resolve(entry, pattern, resolve); ok {
			out = append(out, resolved)
		}
	}
	return out
}

// KeyNames 取 "KEY=VALUE" 条目的键名(日志用, 不泄露值)。
func KeyNames(entries []string) string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if index := strings.Index(entry, "="); index > 0 {
			names = append(names, entry[:index])
		}
	}
	return strings.Join(names, ",")
}

// Values 取 "KEY=VALUE" 条目的值部分(不存在等号时整条视为值)。
func Values(entries []string) []string {
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		if index := strings.Index(entry, "="); index >= 0 {
			values = append(values, entry[index+1:])
			continue
		}
		values = append(values, entry)
	}
	return values
}
