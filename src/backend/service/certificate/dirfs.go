// 本文件是证书目录(位于 store root 之外)的 OS 适配层: 变量展开、原子写、文件名拆解。
package certificate

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	domaincertificate "github.com/wensboy/automan/backend/domain/certificate"
)

// tempPrefix 标记本服务写入时的临时文件, 扫描时跳过。
const tempPrefix = ".automan-tmp-"

// resolveCertDir 展开 ${HOME} 并校验为绝对路径; 其他 ${...} 视为非法(不猜路径)。
func resolveCertDir(raw string) (string, error) {
	dir := strings.TrimSpace(raw)
	if dir == "" {
		return "", errors.New("certificate: certDir is required (conf/certificate.json)")
	}
	if strings.Contains(dir, "${HOME}") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("certificate: resolve home dir: %w", err)
		}
		dir = strings.ReplaceAll(dir, "${HOME}", home)
	}
	if strings.Contains(dir, "${") {
		return "", fmt.Errorf("certificate: unsupported variable in certDir %q", raw)
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("%w: certDir must be absolute", domaincertificate.ErrInvalidPath)
	}
	return filepath.Clean(dir), nil
}

// writeAtomic 同目录临时文件 + rename: 外部目录的原子写(记录文件由 store 引擎保证)。
func writeAtomic(target string, payload []byte) error {
	directory := filepath.Dir(target)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("certificate: prepare target dir: %w", err)
	}
	temp, err := os.CreateTemp(directory, tempPrefix+"*")
	if err != nil {
		return fmt.Errorf("certificate: create temp file: %w", err)
	}
	name := temp.Name()
	if _, err := temp.Write(payload); err != nil {
		temp.Close()
		os.Remove(name)
		return fmt.Errorf("certificate: write temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(name)
		return fmt.Errorf("certificate: close temp file: %w", err)
	}
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return fmt.Errorf("certificate: replace target: %w", err)
	}
	return nil
}

// privateKeyMatchesCertificate 判断私钥能否解出且与该证书成对(公钥一致)。
// 加密私钥需要口令, 本服务没有口令来源, 一律视为不可用。
func privateKeyMatchesCertificate(certificatePayload, keyPayload []byte) bool {
	certificate := parseCertificate(certificatePayload)
	key := parsePrivateKey(keyPayload)
	if certificate == nil || key == nil {
		return false
	}
	return publicKeysEqual(certificate.PublicKey, key.Public())
}

// publicKeysEqual 比较两把公钥(rsa/ecdsa/ed25519 均实现 Equal)。
func publicKeysEqual(left, right crypto.PublicKey) bool {
	comparable, ok := left.(interface{ Equal(crypto.PublicKey) bool })
	if !ok {
		return false
	}
	return comparable.Equal(right)
}

// parseCertificate 取 PEM 首块证书; 非证书内容返回 nil。
func parseCertificate(payload []byte) *x509.Certificate {
	block, _ := pem.Decode(payload)
	if block == nil {
		return nil
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	return certificate
}

// parsePrivateKey 解出 PEM 私钥(PKCS1/PKCS8/EC); 加密或非私钥内容返回 nil。
func parsePrivateKey(payload []byte) crypto.Signer {
	block, _ := pem.Decode(payload)
	if block == nil {
		return nil
	}
	if strings.Contains(strings.ToUpper(block.Type), "ENCRYPTED") || strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED") {
		return nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key
	}
	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if signer, ok := parsed.(crypto.Signer); ok {
			return signer
		}
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key
	}
	return nil
}

// splitCertificateName 拆出(名称, 类型): 类型即合法后缀, 非法后缀返回 false。
func splitCertificateName(base string) (string, string, bool) {
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return "", "", false
	}
	fileType := strings.ToLower(base[dot+1:])
	if !validType(fileType) {
		return "", "", false
	}
	stem := strings.TrimSpace(base[:dot])
	if stem == "" {
		return "", "", false
	}
	return stem, fileType, true
}

func validType(fileType string) bool {
	for _, candidate := range domaincertificate.Types {
		if candidate == fileType {
			return true
		}
	}
	return false
}
