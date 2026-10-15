// Package paths 把逻辑路径解析为用户可写目录下的绝对路径, 由各存储引擎共用.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Resolve 返回 <UserConfigDir>/automan/<logical>.
func Resolve(logical string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("paths: resolve config dir: %w", err)
	}
	return filepath.Join(base, "automan", filepath.FromSlash(logical)), nil
}
