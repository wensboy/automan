// Package config 定义启动期静态配置的领域契约与键 (取值辅助见 internal/kits/configkit).
package config

// Provider 是配置引擎对上暴露的取值能力, key 格式为 namespace.<path>.
type Provider interface {
	Lookup(key string) (any, bool)
}
