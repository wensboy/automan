package config

import "github.com/wensboy/automan/backend/domain/key"

// EnvironmentsKey 取 conf/environments 命名空间。
var EnvironmentsKey = key.New[Environments]("conf/environments")

// Environments 映射 data/conf/environments.json。
type Environments struct {
	// Dir 是相对于 store 根的目录名(默认 "environments")。
	Dir string `json:"dir"`
	// Template 是 data 内嵌目录下的模板路径(默认 "data/default/env.tpl.json")。
	Template string `json:"template"`
	// MaxEntries 限制单个文件允许的条目数(0 表示不限制)。
	MaxEntries int `json:"maxEntries"`
	// AllowRecursiveDelete 是否允许递归删除目录。
	AllowRecursiveDelete bool `json:"allowRecursiveDelete"`
}
