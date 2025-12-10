package utils

import (
	"runtime"
	"strings"
	"testing"
)

// TestGetPlatformInfo 测试获取平台信息
func TestGetPlatformInfo(t *testing.T) {
	info := GetPlatformInfo()

	if info.OS == "" {
		t.Error("OS 不应为空")
	}
	if info.Arch == "" {
		t.Error("Arch 不应为空")
	}

	// 验证与 runtime 包一致
	if info.OS != runtime.GOOS {
		t.Errorf("OS 不匹配: got %s, want %s", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch 不匹配: got %s, want %s", info.Arch, runtime.GOARCH)
	}
}

// TestBuildDownloadURL 测试构建下载链接
func TestBuildDownloadURL(t *testing.T) {
	testCases := []struct {
		name     string
		version  string
		platform PlatformInfo
		contains []string
	}{
		{
			name:     "Linux AMD64",
			version:  "1.0.0",
			platform: PlatformInfo{OS: "linux", Arch: "amd64"},
			contains: []string{"v1.0.0", "linux", "amd64", ".tar.gz"},
		},
		{
			name:     "macOS ARM64",
			version:  "1.2.3",
			platform: PlatformInfo{OS: "darwin", Arch: "arm64"},
			contains: []string{"v1.2.3", "darwin", "arm64", ".tar.gz"},
		},
		{
			name:     "Windows AMD64",
			version:  "2.0.0",
			platform: PlatformInfo{OS: "windows", Arch: "amd64"},
			contains: []string{"v2.0.0", "windows", "amd64", ".zip"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := BuildDownloadURL(tc.version, tc.platform)

			for _, s := range tc.contains {
				if !strings.Contains(url, s) {
					t.Errorf("URL 应该包含 %q: %s", s, url)
				}
			}
		})
	}
}

// TestCompareVersions 测试版本号比较
func TestCompareVersions(t *testing.T) {
	testCases := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// 相等版本
		{"相等版本", "1.0.0", "1.0.0", 0},
		{"相等版本带v前缀", "v1.0.0", "v1.0.0", 0},
		{"相等版本混合前缀", "v1.0.0", "1.0.0", 0},

		// v1 < v2
		{"主版本小于", "1.0.0", "2.0.0", -1},
		{"次版本小于", "1.1.0", "1.2.0", -1},
		{"补丁版本小于", "1.0.1", "1.0.2", -1},
		{"较短版本小于", "1.0", "1.0.1", -1},

		// v1 > v2
		{"主版本大于", "2.0.0", "1.0.0", 1},
		{"次版本大于", "1.2.0", "1.1.0", 1},
		{"补丁版本大于", "1.0.2", "1.0.1", 1},
		{"较长版本大于", "1.0.1", "1.0", 1},

		// 边界情况
		{"空版本比较", "", "", 0},
		{"单数字版本", "1", "2", -1},
		{"双数字版本", "1.1", "1.2", -1},
		{"多位数字", "1.10.0", "1.9.0", 1},
		{"dev版本", "dev", "1.0.0", -1}, // dev 解析为 0
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := compareVersions(tc.v1, tc.v2)
			if result != tc.expected {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tc.v1, tc.v2, result, tc.expected)
			}
		})
	}
}

// TestCompareVersions_Symmetry 测试版本比较的对称性
func TestCompareVersions_Symmetry(t *testing.T) {
	pairs := []struct {
		v1, v2 string
	}{
		{"1.0.0", "2.0.0"},
		{"1.1.0", "1.2.0"},
		{"1.0.1", "1.0.2"},
	}

	for _, p := range pairs {
		r1 := compareVersions(p.v1, p.v2)
		r2 := compareVersions(p.v2, p.v1)

		if r1 == 0 && r2 != 0 {
			t.Errorf("对称性错误: compareVersions(%q, %q) = %d, 但 compareVersions(%q, %q) = %d",
				p.v1, p.v2, r1, p.v2, p.v1, r2)
		}
		if r1 < 0 && r2 <= 0 {
			t.Errorf("对称性错误: compareVersions(%q, %q) = %d, compareVersions(%q, %q) 应该 > 0",
				p.v1, p.v2, r1, p.v2, p.v1)
		}
		if r1 > 0 && r2 >= 0 {
			t.Errorf("对称性错误: compareVersions(%q, %q) = %d, compareVersions(%q, %q) 应该 < 0",
				p.v1, p.v2, r1, p.v2, p.v1)
		}
	}
}

// TestPlatformInfo_Struct 测试 PlatformInfo 结构体
func TestPlatformInfo_Struct(t *testing.T) {
	info := PlatformInfo{
		OS:   "linux",
		Arch: "amd64",
	}

	if info.OS != "linux" {
		t.Errorf("OS 错误: got %s, want linux", info.OS)
	}
	if info.Arch != "amd64" {
		t.Errorf("Arch 错误: got %s, want amd64", info.Arch)
	}
}

// TestVersionCheckResult_Struct 测试 VersionCheckResult 结构体
func TestVersionCheckResult_Struct(t *testing.T) {
	result := VersionCheckResult{
		CurrentVersion: "1.0.0",
		LatestVersion:  "1.1.0",
		HasUpdate:      true,
		ReleaseURL:     "https://github.com/example/releases/v1.1.0",
		ReleaseNotes:   "New features",
		PublishedAt:    "2024-01-01T00:00:00Z",
		Platform: PlatformInfo{
			OS:   "linux",
			Arch: "amd64",
		},
		DownloadURL: "https://github.com/example/download/v1.1.0/file.tar.gz",
	}

	if result.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion 错误")
	}
	if result.LatestVersion != "1.1.0" {
		t.Errorf("LatestVersion 错误")
	}
	if !result.HasUpdate {
		t.Errorf("HasUpdate 应该为 true")
	}
}

// TestGitHubRelease_Struct 测试 GitHubRelease 结构体
func TestGitHubRelease_Struct(t *testing.T) {
	release := GitHubRelease{
		TagName:     "v1.0.0",
		Name:        "Release 1.0.0",
		Body:        "Release notes",
		HTMLURL:     "https://github.com/example/releases/v1.0.0",
		PublishedAt: "2024-01-01T00:00:00Z",
		Prerelease:  false,
		Draft:       false,
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("TagName 错误")
	}
	if release.Draft || release.Prerelease {
		t.Errorf("Draft 和 Prerelease 应该为 false")
	}
}

// BenchmarkCompareVersions 基准测试版本比较
func BenchmarkCompareVersions(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compareVersions("1.2.3", "1.2.4")
	}
}

// BenchmarkBuildDownloadURL 基准测试构建下载链接
func BenchmarkBuildDownloadURL(b *testing.B) {
	platform := PlatformInfo{OS: "linux", Arch: "amd64"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildDownloadURL("1.0.0", platform)
	}
}
