// 本文件负责扫描证书目录、解析 PEM 有效期, 并计算记录差异。
package certificate

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	domaincertificate "github.com/wensboy/automan/backend/domain/certificate"
)

// certFile 是证书目录里扫到的一个合法证书文件(私钥不在此列)。
type certFile struct {
	// path 是相对证书目录的文件名。
	path     string
	name     string
	fileType string
	valid    string
}

// scanDir 扫描证书目录: 跳过子目录、隐藏文件(含临时文件)与非法后缀。
// scanDir 返回每个证书文件可用的私钥集合(certPath -> 私钥文件名集合):
// 私钥必须与该证书公钥成对才算可用, 不单独成条; 无法配对的私钥计入 skipped。
func scanDir(dir string) ([]certFile, map[string]map[string]bool, domaincertificate.Stats, error) {
	var stats domaincertificate.Stats
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, stats, fmt.Errorf("certificate: read cert dir: %w", err)
	}

	keyFiles := make([]string, 0, len(entries))
	files := make([]certFile, 0, len(entries))
	for _, entry := range entries {
		base := entry.Name()
		if entry.IsDir() || strings.HasPrefix(base, ".") {
			continue // 子目录 / 隐藏文件 / .automan-tmp-* 临时文件
		}
		stats.Scanned++
		name, fileType, ok := splitCertificateName(base)
		if !ok {
			stats.Skipped++
			continue
		}
		if fileType == "key" {
			keyFiles = append(keyFiles, base)
			continue
		}
		files = append(files, certFile{path: base, name: name, fileType: fileType})
	}

	matches := make(map[string]map[string]bool, len(files))
	publicKeys := make(map[string]crypto.PublicKey, len(keyFiles))
	paired := 0
	for index := range files {
		payload, err := os.ReadFile(filepath.Join(dir, files[index].path))
		if err != nil {
			continue
		}
		files[index].valid = certificateValid(payload, files[index].fileType)
		certificate := parseCertificate(payload)
		if certificate == nil {
			continue
		}
		available := make(map[string]bool, 1)
		for _, keyName := range keyFiles {
			keyPayload, readErr := os.ReadFile(filepath.Join(dir, keyName))
			if readErr != nil {
				continue
			}
			publicKey, ok := publicKeys[keyName]
			if !ok {
				signer := parsePrivateKey(keyPayload)
				if signer == nil {
					continue
				}
				publicKey = signer.Public()
				publicKeys[keyName] = publicKey
			}
			if publicKeysEqual(certificate.PublicKey, publicKey) {
				available[keyName] = true
			}
		}
		paired += len(available)
		matches[files[index].path] = available
	}
	stats.Skipped += len(keyFiles) - paired // 孤立私钥 / 与证书不匹配的私钥
	return files, matches, stats, nil
}

// certificateValid 解析 PEM 首块证书取 notAfter; 私钥或非证书内容留空。
func certificateValid(payload []byte, fileType string) string {
	if fileType == "key" {
		return ""
	}
	block, _ := pem.Decode(payload)
	if block == nil {
		return ""
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return ""
	}
	return certificate.NotAfter.UTC().Format(time.RFC3339)
}

// mergeRecords 按 certPath 对比扫描结果与现有记录: 新增/移除/元数据刷新, 返回是否有变化。
// 双指针压缩: 读指针遍历现有记录(已按 id 有序), 写指针就地压缩出保留项(顺序不变),
// 扫描到的新文件追加到尾部, 最后把 id 重排成连续的 1..n(不再保留历史空洞)。
// keyPath 允许用户手工指定: 指向的私钥仍存在且与本证书配对就保留, 失效则回落到同名 .key。
func mergeRecords(
	current []domaincertificate.Record,
	files []certFile,
	matches map[string]map[string]bool,
	now time.Time,
	stats domaincertificate.Stats,
) ([]domaincertificate.Record, bool, domaincertificate.Stats) {
	filesByPath := make(map[string]certFile, len(files))
	for _, file := range files {
		filesByPath[file.path] = file
	}

	stamp := now.Format(time.RFC3339)
	next := make([]domaincertificate.Record, 0, len(files))
	seen := make(map[string]bool, len(files))
	changed := false

	for _, record := range current {
		file, ok := filesByPath[record.CertPath]
		if !ok {
			stats.Removed++
			changed = true
			continue
		}
		seen[record.CertPath] = true
		available := matches[file.path]
		keyPath := record.KeyPath
		if !available[keyPath] {
			keyPath = ""
		}
		if keyPath == "" && available[file.name+".key"] {
			keyPath = file.name + ".key"
		}
		if record.Valid != file.valid || record.KeyPath != keyPath {
			record.Valid = file.valid
			record.KeyPath = keyPath
			record.UpdatedAt = stamp
			stats.Updated++
			changed = true
		}
		next = append(next, record)
	}

	for _, file := range files {
		if seen[file.path] {
			continue
		}
		keyPath := ""
		if matches[file.path][file.name+".key"] {
			keyPath = file.name + ".key"
		}
		next = append(next, domaincertificate.Record{
			Name:      file.name,
			Type:      file.fileType,
			CreatedAt: stamp,
			UpdatedAt: stamp,
			CertPath:  file.path,
			KeyPath:   keyPath,
			Valid:     file.valid,
			Tags:      []string{},
		})
		stats.Added++
		changed = true
	}

	// 双指针压缩后的重排: id 连续, 顺序即压缩后的顺序
	for index := range next {
		if next[index].ID != index+1 {
			next[index].ID = index + 1
			changed = true
		}
	}
	return next, changed, stats
}
