package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateID 生成随机ID
func GenerateID(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}
