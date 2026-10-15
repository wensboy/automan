package config

import "github.com/wensboy/automan/backend/domain/key"

// Error 映射 data/conf/error.json (meta 除外).
type Error struct {
	Builtin []ErrorItem `json:"builtin"`
	Service []ErrorItem `json:"service"`
}

// ErrorItem 是单条错误定义.
type ErrorItem struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// ErrorKey 取全部错误定义 (namespace 根).
var ErrorKey = key.New[Error]("conf/error")
