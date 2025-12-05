package utils

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// GenerateID 生成随机ID
func GenerateID(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		// 如果加密随机数生成失败，使用时间戳作为后备方案
		// 这种情况非常罕见，但不应该返回全零
		timestamp := time.Now().UnixNano()
		for i := 0; i < length && i < 8; i++ {
			b[i] = byte(timestamp >> (i * 8))
		}
	}
	return hex.EncodeToString(b)
}
