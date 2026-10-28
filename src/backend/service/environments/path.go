// 本文件负责路径规范化与字符集校验。
package environments

import (
	"path"
	"regexp"
	"strings"

	domainenvironments "github.com/wensboy/automan/backend/domain/environments"
)

var pathPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// --- 校验/模板/解析 ---

func normalizeDir(rel string) (string, error) {
	cleaned := strings.Trim(strings.TrimSpace(rel), "/")
	if cleaned == "" || strings.Contains(cleaned, "..") || !pathPattern.MatchString(cleaned) {
		return "", domainenvironments.ErrInvalidPath
	}
	if strings.HasSuffix(strings.ToLower(cleaned), ".json") {
		return "", domainenvironments.ErrInvalidPath
	}
	return path.Clean(cleaned), nil
}

func normalizeFile(rel string) (string, error) {
	cleaned := strings.Trim(strings.TrimSpace(rel), "/")
	if cleaned == "" || strings.Contains(cleaned, "..") || !pathPattern.MatchString(cleaned) {
		return "", domainenvironments.ErrInvalidPath
	}
	if !strings.HasSuffix(strings.ToLower(cleaned), ".json") || strings.HasSuffix(cleaned, "/") {
		return "", domainenvironments.ErrInvalidPath
	}
	return path.Clean(cleaned), nil
}
