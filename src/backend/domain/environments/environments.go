// Package environments 是"统一环境变量"的领域契约(见 docs/design/v26.md)。
package environments

import "errors"

// 节点类型。
const (
	NodeDir  = "dir"
	NodeFile = "file"
)

// 支持的 value 类型。
var Types = []string{"int", "bool", "float", "string", "array", "object"}

// 领域错误(api 层映射为 apperr)。
var (
	ErrInvalidPath  = errors.New("environments: invalid path")
	ErrExists       = errors.New("environments: already exists")
	ErrNotFound     = errors.New("environments: not found")
	ErrDirNotEmpty  = errors.New("environments: directory not empty")
	ErrInvalidFile  = errors.New("environments: invalid env file")
	ErrUnknownPlace = errors.New("environments: unknown template placeholder")
	ErrConflict     = errors.New("environments: duplicate key across files")
	ErrRecursive    = errors.New("environments: recursive delete is disabled")
)

// Node 是树上的一个节点(路径为相对 environments 根)。
type Node struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Children []Node `json:"children,omitempty"`
}

// Entry 是文件内的一个键值条目。
type Entry struct {
	Type  string `json:"type"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Document 是单个挂载点文件的内容。
type Document struct {
	Path    string            `json:"path"`
	Name    string            `json:"name"`
	Content string            `json:"content"`
	Entry   string            `json:"entry"`
	Entries []Entry           `json:"entries"`
	Values  map[string]string `json:"values"`
	Meta    map[string]string `json:"meta"`
}

// Provider 供其它服务注入取值(方法值, 避免 import 具体 service)。
type Provider interface {
	Values(paths ...string) (map[string]string, error)
}
