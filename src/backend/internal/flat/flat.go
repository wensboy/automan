// Package flat 提供嵌套 JSON 对象与点号平铺键之间的互转.
package flat

import "strings"

// Tree 是嵌套的 JSON 对象 (叶子为标量或数组).
type Tree = map[string]any

// Paths 把 root 展开为 "prefix.<path>" 形式的叶子键写入 dst; prefix 为空时直接使用路径.
func Paths(dst map[string]any, prefix string, root Tree) {
	for name, value := range root {
		key := name
		if prefix != "" {
			key = prefix + "." + name
		}
		if child, ok := value.(Tree); ok {
			Paths(dst, key, child)
			continue
		}
		dst[key] = value
	}
}

// Build 把点号叶子键还原为嵌套对象.
func Build(paths map[string]any) Tree {
	root := Tree{}
	for key, value := range paths {
		names := strings.Split(key, ".")
		node := root
		for _, name := range names[:len(names)-1] {
			child, ok := node[name].(Tree)
			if !ok {
				child = Tree{}
				node[name] = child
			}
			node = child
		}
		node[names[len(names)-1]] = value
	}
	return root
}
