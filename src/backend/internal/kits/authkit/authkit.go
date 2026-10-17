// Package authkit 提供口令与令牌的底层原语: bcrypt 哈希、随机令牌、令牌摘要.
// 只放纯函数, 不涉及存储与业务(v18 §11 B: 采用标准做法 bcrypt).
package authkit

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLen 是口令的最小长度(与前端既有提示一致).
const MinPasswordLen = 8

// MaxPasswordLen 是口令的最大长度(bcrypt 只取前 72 字节, 这里做输入约束).
const MaxPasswordLen = 256

// HashPassword 返回口令的 bcrypt 哈希.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// VerifyPassword 校验明文口令与哈希是否匹配.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// NewToken 生成 128bit 随机会话令牌(hex 32 字符).
func NewToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", errors.New("authkit: random source unavailable")
	}
	return hex.EncodeToString(buf), nil
}

// HashToken 返回令牌的 SHA-256 摘要: 落库只存它, 不存明文令牌.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
