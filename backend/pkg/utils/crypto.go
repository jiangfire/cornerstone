package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashString 计算字符串的SHA256哈希
func HashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}