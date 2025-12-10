package config

// Version 版本号，编译时通过 -ldflags 注入
// 本地开发时显示 dev
var Version = "dev"

// GitCommit Git 提交哈希，编译时通过 -ldflags 注入
var GitCommit = ""

// BuildTime 构建时间，编译时通过 -ldflags 注入
var BuildTime = ""

// FullVersion 返回完整版本信息
func FullVersion() string {
	if GitCommit != "" {
		return Version + " (" + GitCommit[:7] + ")"
	}
	return Version
}
