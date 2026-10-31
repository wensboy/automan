// 本文件负责模板渲染与"编辑保存"的元数据校验。
package certificate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	domaincertificate "github.com/wensboy/automan/backend/domain/certificate"
)

// templateData 是证书文档模板的渲染上下文(text/template 语义, 模板见 data/default/certificate.tpl.json)。
type templateData struct {
	Name      string
	Type      string
	CreatedAt string
	UpdatedAt string
	CertPath  string
	KeyPath   string
	Valid     string
}

// documentKeys 是模板里出现的键, 编辑保存时据此判定未知键。
var documentKeys = []string{"name", "type", "createdAt", "updatedAt", "certPath", "keyPath", "valid", "tags"}

// render 用当前记录渲染模板; 结果是展示给用户的原文(记录才是结构化事实)。
func (s *Service) render(record domaincertificate.Record) (string, error) {
	payload, err := fs.ReadFile(s.data, s.template)
	if err != nil {
		return "", fmt.Errorf("certificate: read template: %w", err)
	}
	tpl, err := template.New(path.Base(s.template)).Parse(string(payload))
	if err != nil {
		return "", fmt.Errorf("%w: %v", domaincertificate.ErrInvalidFile, err)
	}
	var buffer bytes.Buffer
	if err := tpl.Execute(&buffer, templateData{
		Name:      record.Name,
		Type:      record.Type,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
		CertPath:  record.CertPath,
		KeyPath:   record.KeyPath,
		Valid:     record.Valid,
	}); err != nil {
		return "", fmt.Errorf("%w: %v", domaincertificate.ErrUnknownPlace, err)
	}
	if !json.Valid(buffer.Bytes()) {
		return "", fmt.Errorf("%w: rendered template is not valid JSON", domaincertificate.ErrInvalidFile)
	}
	content, err := applyTags(buffer.String(), record.Tags)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domaincertificate.ErrInvalidFile, err)
	}
	return content, nil
}

// applyTags 用记录里的标签覆盖渲染结果中的 tags, 其余键保持模板顺序。
// 模板里没有 tags 键时不强行添加(模板是用户定义内容)。
func applyTags(content string, tags []string) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(content))
	if _, err := decoder.Token(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(tags)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("{\n")
	first := true
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		key, ok := token.(string)
		if !ok {
			return "", errors.New("key is not a string")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return "", err
		}
		if key == "tags" {
			value = payload
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, value); err != nil {
			return "", err
		}
		keyText, err := json.Marshal(key)
		if err != nil {
			return "", err
		}
		if !first {
			builder.WriteString(",\n")
		}
		first = false
		builder.WriteString("    ")
		builder.Write(keyText)
		builder.WriteString(": ")
		builder.Write(compact.Bytes())
	}
	builder.WriteString("\n}")
	return builder.String(), nil
}

func (s *Service) document(record domaincertificate.Record) (domaincertificate.Document, error) {
	content, err := s.render(record)
	if err != nil {
		return domaincertificate.Document{}, err
	}
	return domaincertificate.Document{Record: record, Content: content}, nil
}

// applyDocument 从编辑区原文提取可改键(name/tags/valid/keyPath); 只读键被改动或出现未知键时报错。
// updatedAt 由服务端维护, 原文里的取值不参与比较; keyPath 必须是证书目录内真实存在的私钥。
func (s *Service) applyDocument(record domaincertificate.Record, content string) (domaincertificate.Record, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &document); err != nil {
		return record, fmt.Errorf("%w: %v", domaincertificate.ErrInvalidFile, err)
	}
	for key := range document {
		if !knownDocumentKey(key) {
			return record, fmt.Errorf("%w: unknown key %q", domaincertificate.ErrInvalidFile, key)
		}
	}

	for _, readonly := range []struct{ key, want string }{
		{"type", record.Type},
		{"createdAt", record.CreatedAt},
		{"certPath", record.CertPath},
	} {
		if err := matchString(document, readonly.key, readonly.want); err != nil {
			return record, err
		}
	}

	if raw, ok := document["name"]; ok {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return record, fmt.Errorf("%w: name must be a string", domaincertificate.ErrInvalidFile)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return record, fmt.Errorf("%w: name is required", domaincertificate.ErrInvalidFile)
		}
		record.Name = name
	}
	if raw, ok := document["valid"]; ok {
		var valid string
		if err := json.Unmarshal(raw, &valid); err != nil {
			return record, fmt.Errorf("%w: valid must be a string", domaincertificate.ErrInvalidFile)
		}
		record.Valid = strings.TrimSpace(valid)
	}
	if raw, ok := document["keyPath"]; ok {
		var keyPath string
		if err := json.Unmarshal(raw, &keyPath); err != nil {
			return record, fmt.Errorf("%w: keyPath must be a string", domaincertificate.ErrInvalidFile)
		}
		resolved, err := s.attachKeyPath(record, strings.TrimSpace(keyPath))
		if err != nil {
			return record, err
		}
		record.KeyPath = resolved
	}
	if raw, ok := document["tags"]; ok {
		var tags []string
		if err := json.Unmarshal(raw, &tags); err != nil {
			return record, fmt.Errorf("%w: tags must be a string array", domaincertificate.ErrInvalidFile)
		}
		if tags == nil {
			tags = []string{}
		}
		record.Tags = tags
	}
	return record, nil
}

// attachKeyPath 校验编辑区填写的 keyPath 并返回记录里最终保存的值:
//
//	· 空串 -> 解除配对(不删除任何文件);
//	· 相对路径 -> 视为证书目录内的私钥, 解出私钥后直接沿用;
//	· 绝对路径(任意位置) -> 解出私钥后复制到证书目录, 命名为与证书同名的 .key。
func (s *Service) attachKeyPath(record domaincertificate.Record, keyPath string) (string, error) {
	if keyPath == "" {
		return "", nil
	}
	source := keyPath
	if !filepath.IsAbs(keyPath) {
		if strings.Contains(keyPath, "..") || strings.HasPrefix(keyPath, ".") {
			return "", fmt.Errorf("%w: keyPath %q escapes the cert dir", domaincertificate.ErrInvalidFile, keyPath)
		}
		source = filepath.Join(s.certDir, keyPath)
	}
	payload, err := os.ReadFile(source)
	if err != nil {
		return "", fmt.Errorf("%w: keyPath %q cannot be read", domaincertificate.ErrInvalidFile, keyPath)
	}
	certificatePayload, err := os.ReadFile(filepath.Join(s.certDir, record.CertPath))
	if err != nil {
		return "", fmt.Errorf("certificate: read certificate: %w", err)
	}
	if !privateKeyMatchesCertificate(certificatePayload, payload) {
		return "", fmt.Errorf("%w: keyPath %q does not match %s", domaincertificate.ErrInvalidFile, keyPath, record.CertPath)
	}
	if !filepath.IsAbs(keyPath) {
		return keyPath, nil
	}

	target := strings.TrimSuffix(record.CertPath, path.Ext(record.CertPath)) + ".key"
	destination := filepath.Join(s.certDir, target)
	if existing, readErr := os.ReadFile(destination); readErr == nil {
		if bytes.Equal(existing, payload) {
			return target, nil
		}
		return "", fmt.Errorf("%w: %s already exists with different content", domaincertificate.ErrExists, target)
	}
	if err := writeAtomic(destination, payload); err != nil {
		return "", err
	}
	return target, nil
}

func knownDocumentKey(key string) bool {
	for _, candidate := range documentKeys {
		if candidate == key {
			return true
		}
	}
	return false
}

// matchString 校验只读键: 缺失视为未改, 出现且取值不同即拒绝。
func matchString(document map[string]json.RawMessage, key, want string) error {
	raw, ok := document[key]
	if !ok {
		return nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("%w: %s must be a string", domaincertificate.ErrInvalidFile, key)
	}
	if value != want {
		return fmt.Errorf("%w: %s is read-only", domaincertificate.ErrInvalidFile, key)
	}
	return nil
}
