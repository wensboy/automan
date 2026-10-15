package config

import "github.com/wensboy/automan/backend/domain/key"

// Store 映射 data/conf/store.json (meta 除外); 缺省项由 store 引擎内置.
type Store struct {
	Root      string   `json:"root"`
	Shards    int      `json:"shards"`
	Inflight  int      `json:"inflight"`
	Queue     int      `json:"queue"`
	TimeoutMs int      `json:"timeoutMs"`
	MaxSize   int      `json:"maxSize"`
	Database  Database `json:"database"`
}

// StoreKey 取整个 store 配置 (namespace 根).
var StoreKey = key.New[Store]("conf/store")
