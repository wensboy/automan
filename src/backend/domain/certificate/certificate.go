// Package certificate 是"全局证书管理"的领域契约(见 docs/design/v27.md)。
package certificate

import "errors"

// Types 是合法证书类型, 即文件后缀。
var Types = []string{"crt", "pem", "key"}

// 领域错误(api 层直接透传文案)。
var (
	ErrInvalidPath  = errors.New("certificate: invalid path")
	ErrNotFound     = errors.New("certificate: not found")
	ErrExists       = errors.New("certificate: target already exists")
	ErrInvalidCert  = errors.New("certificate: unsupported certificate file")
	ErrLimit        = errors.New("certificate: record limit exceeded")
	ErrInvalidFile  = errors.New("certificate: invalid record document")
	ErrUnknownPlace = errors.New("certificate: unknown template placeholder")
)

// Record 是一条证书记录: 一个证书文件一条, 路径相对证书目录。
type Record struct {
	// ID 自增且不复用。
	ID int `json:"id"`
	// Name 允许重名; 导入时取自去掉后缀的文件名。
	Name string `json:"name"`
	// Type 即文件后缀(crt|pem|key)。
	Type string `json:"type"`
	// CreatedAt / UpdatedAt 为 RFC3339 时间。
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	// CertPath 是相对证书目录的文件名。
	CertPath string `json:"certPath"`
	// KeyPath 是同一份证书的私钥(同名 .key), 不存在时为空。
	KeyPath string `json:"keyPath"`
	// Valid 是证书 notAfter(RFC3339), 无法解析时为空。
	Valid string `json:"valid"`
	// Tags 为用户自定义标签。
	Tags []string `json:"tags"`
}

// Snapshot 是全量记录快照(按 ID 升序)。
type Snapshot struct {
	SyncedAt string   `json:"syncedAt"`
	Records  []Record `json:"records"`
}

// Document 是 UI 的展示单元: 记录 + 模板渲染后的原文。
type Document struct {
	Record  Record `json:"record"`
	Content string `json:"content"`
}

// Stats 是一次同步的统计(只进 log 与事件)。
type Stats struct {
	Scanned int `json:"scanned"`
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}
