// Package about 汇总"程序自身"信息(见 docs/design/v25.md §15/§17)。
//
// 唯一数据来源: conf/about(application 相关字段已并入 about.json) + 构建注入兜底。
// 前端只做显示映射(label/格式化/链接), 不再自行计算值。
package about

import (
	domainconfig "github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/key"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
)

// Properties 是展示所需的属性值(与 conf/about.properties 一一对应)。
type Properties struct {
	Version    string `json:"version"`
	BuildTime  string `json:"buildTime"`
	BuildHash  string `json:"buildHash"`
	License    string `json:"license"`
	LicenseURL string `json:"licenseUrl"`
	Repository string `json:"repository"`
}

// Info 是 about 页面的完整信息。
type Info struct {
	AppName     string     `json:"appName"`
	Description string     `json:"description"`
	Properties  Properties `json:"properties"`
}

// Service 提供 about 页面所需信息。
type Service struct {
	provider domainconfig.Provider
}

// Key 是服务注册键。
var Key = key.New[*Service]("about")

// New 创建服务(无参: 依赖通过 With* 注入)。
func New() *Service { return &Service{} }

// WithConfig 注入配置读取器(conf/about)。
func (s *Service) WithConfig(provider domainconfig.Provider) *Service {
	s.provider = provider
	return s
}

// Info 返回合并后的信息: 配置非空优先, 构建注入兜底。
func (s *Service) Info() Info {
	cfg, _ := configkit.Get[domainconfig.About](s.provider, domainconfig.AboutKey)
	build := domainconfig.Build()

	license, licenseURL := "", ""
	if len(cfg.Properties.License) > 0 {
		license = cfg.Properties.License[0]
	}
	if len(cfg.Properties.License) > 1 {
		licenseURL = cfg.Properties.License[1]
	}

	return Info{
		AppName:     pick(cfg.AppName, "Automan"),
		Description: cfg.Description,
		Properties: Properties{
			Version:    pick(cfg.Properties.Version, build.Version),
			BuildTime:  pick(cfg.Properties.BuildTime, build.BuildTime),
			BuildHash:  pick(cfg.Properties.BuildHash, build.BuildHash),
			License:    license,
			LicenseURL: licenseURL,
			Repository: cfg.Properties.Repository,
		},
	}
}

// pick 返回第一个非空值(配置优先, 构建注入兜底)。
func pick(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
