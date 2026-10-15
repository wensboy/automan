package config

// Database 是 database 引擎的参数, 存放于 conf/store.database.
type Database struct {
	Driver        string `json:"driver"`
	Path          string `json:"path"`
	Shards        int    `json:"shards"`
	Inflight      int    `json:"inflight"`
	Queue         int    `json:"queue"`
	TimeoutMs     int    `json:"timeoutMs"`
	BusyTimeoutMs int    `json:"busyTimeoutMs"`
	Journal       string `json:"journal"`
	SlowMs        int    `json:"slowMs"`
	MaxRows       int    `json:"maxRows"`
	LogArgs       bool   `json:"logArgs"`
}
