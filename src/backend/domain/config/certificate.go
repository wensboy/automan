package config

import "github.com/wensboy/automan/backend/domain/key"

// CertificateKey 取 conf/certificate 命名空间。
var CertificateKey = key.New[Certificate]("conf/certificate")

// Certificate 映射 data/conf/certificate.json。
type Certificate struct {
	// Root 是记录文件相对 store 根的目录(默认 "certs")。
	Root string `json:"root"`
	// File 是记录文件名(默认 "certificates.json")。
	File string `json:"file"`
	// Limit 限制记录条数(0 表示取缺省 256)。
	Limit int `json:"limit"`
	// CertDir 是 whistle 的全局证书目录, 支持 ${HOME}。
	CertDir string `json:"certDir"`
	// SyncIntervalMs 是后台同步周期(默认 600000, 下限 60000)。
	SyncIntervalMs int `json:"syncIntervalMs"`
}
