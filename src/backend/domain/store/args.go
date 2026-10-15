package store

import "time"

// FileInfo 是文件元信息.
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// ReadArgs 是 fs.read 的参数.
type ReadArgs struct {
	Path string
}

// Key 实现 Keyed.
func (a ReadArgs) Key() string { return a.Path }

// WriteArgs 是 fs.write 的参数, 按原子写替换整个文件.
type WriteArgs struct {
	Path string
	Data []byte
}

// Key 实现 Keyed.
func (a WriteArgs) Key() string { return a.Path }

// AppendArgs 是 fs.append 的参数, 同路径有序追加.
type AppendArgs struct {
	Path string
	Data []byte
}

// Key 实现 Keyed.
func (a AppendArgs) Key() string { return a.Path }

// RemoveArgs 是 fs.remove 的参数.
type RemoveArgs struct {
	Path string
}

// Key 实现 Keyed.
func (a RemoveArgs) Key() string { return a.Path }

// MkdirArgs 是 fs.mkdir 的参数, 创建目录及其父级.
type MkdirArgs struct {
	Path string
}

// Key 实现 Keyed.
func (a MkdirArgs) Key() string { return a.Path }

// StatArgs 是 fs.stat 的参数.
type StatArgs struct {
	Path string
}

// Key 实现 Keyed.
func (a StatArgs) Key() string { return a.Path }

// ListArgs 是 fs.list 的参数.
type ListArgs struct {
	Dir string
}

// Key 实现 Keyed.
func (a ListArgs) Key() string { return a.Dir }
