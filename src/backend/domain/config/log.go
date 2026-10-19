package config

import "github.com/wensboy/automan/backend/domain/key"

// Log 映射 data/conf/log.json (meta 除外); 缺省项由 log 引擎内置.
type Log struct {
	Dir          string `json:"dir"`
	File         string `json:"file"`
	Buffer       int    `json:"buffer"`
	Batch        int    `json:"batch"`
	ConsoleLevel string `json:"consoleLevel"`
	FileLevel    string `json:"fileLevel"`
	// MaxSize 是日志文件最大体积 (MiB): 文件按该上限循环, 超出时保留最新一段.
	MaxSize int `json:"maxSize"`
	// Stream 相关: 送往前端终端的批量窗口与缓冲上限 (见 docs/design/v16.md 第 3 节).
	StreamIntervalMs int `json:"streamIntervalMs"`
	StreamBatch      int `json:"streamBatch"`
	StreamQueue      int `json:"streamQueue"`
	// NotifyCapacity 是运行时提示队列的容量(内存环形, 超出丢最旧; 见 docs/design/v20.md §7 A).
	NotifyCapacity int `json:"notifyCapacity"`
}

// LogKey 取整个 log 配置 (namespace 根).
var LogKey = key.New[Log]("conf/log")
