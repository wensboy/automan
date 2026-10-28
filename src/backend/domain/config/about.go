package config

import (
	"runtime/debug"

	"github.com/wensboy/automan/backend/domain/key"
)

// AboutKey 取 conf/about 命名空间。
var AboutKey = key.New[About]("conf/about")

// 构建期注入(构建脚本通过 -ldflags -X 写入; 未注入时回落到 go build info):
//
//	-X github.com/wensboy/automan/backend/domain/config.buildVersion=<tag>
//	-X github.com/wensboy/automan/backend/domain/config.buildTime=<RFC3339>
//	-X github.com/wensboy/automan/backend/domain/config.buildHash=<short sha>
var (
	buildVersion string
	buildTime    string
	buildHash    string
)

// About 映射 data/conf/about.json(程序自身信息的唯一来源)。
type About struct {
	AppName     string          `json:"appName"`
	Description string          `json:"description"`
	Properties  AboutProperties `json:"properties"`
}

// AboutProperties 是 about 页面的属性集合; License 为 [名称, 链接] 二元组。
type AboutProperties struct {
	Version    string   `json:"version"`
	BuildTime  string   `json:"buildTime"`
	BuildHash  string   `json:"buildHash"`
	License    []string `json:"license"`
	Repository string   `json:"repository"`
}

// BuildInfo 是构建信息(注入优先, 缺失回落 go build info)。
type BuildInfo struct {
	Version   string
	BuildTime string
	BuildHash string
}

// Build 返回构建信息。
func Build() BuildInfo {
	result := BuildInfo{Version: buildVersion, BuildTime: buildTime, BuildHash: buildHash}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return result
	}
	if result.Version == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		result.Version = info.Main.Version
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if result.BuildHash == "" && setting.Value != "" {
				result.BuildHash = shortHash(setting.Value)
			}
		case "vcs.time":
			if result.BuildTime == "" {
				result.BuildTime = setting.Value
			}
		}
	}
	return result
}

func shortHash(value string) string {
	if len(value) > 7 {
		return value[:7]
	}
	return value
}
