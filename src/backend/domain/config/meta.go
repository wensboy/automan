package config

// Meta 是所有配置文件一致的元信息, namespace 决定该文件的键前缀.
type Meta struct {
	Namespace string `json:"namespace"`
}
