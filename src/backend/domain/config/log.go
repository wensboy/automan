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
}

// LogKey 取整个 log 配置 (namespace 根).
var LogKey = key.New[Log]("conf/log")
