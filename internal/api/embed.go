//go:build embed
// +build embed

package api

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var embeddedStatic embed.FS

func init() {
	// 获取 static 子目录作为根目录
	subFS, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}
	staticFS = subFS
	useEmbed = true
}
