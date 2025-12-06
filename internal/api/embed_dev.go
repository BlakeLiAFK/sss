//go:build !embed
// +build !embed

package api

import (
	"os"
)

func init() {
	// 开发模式：从文件系统读取静态文件
	staticFS = os.DirFS("./data/static")
	useEmbed = false
}
