package config

import "github.com/wensboy/automan/backend/domain/key"

// Application 映射 data/conf/application.json (meta 除外).
type Application struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

var (
	// ApplicationKey 取整个应用元信息 (namespace 根).
	ApplicationKey = key.New[Application]("conf/application")
	// ApplicationNameKey 取平铺后的应用名.
	ApplicationNameKey = key.New[string]("conf/application.name")
	// ApplicationVersionKey 取平铺后的版本号.
	ApplicationVersionKey = key.New[string]("conf/application.version")
)
