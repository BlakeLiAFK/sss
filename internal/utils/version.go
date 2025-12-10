package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sss/internal/config"
)

// GitHubRelease GitHub Release 信息
type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
}

// VersionCheckResult 版本检测结果
type VersionCheckResult struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	ReleaseURL     string `json:"release_url,omitempty"`
	ReleaseNotes   string `json:"release_notes,omitempty"`
	PublishedAt    string `json:"published_at,omitempty"`
}

const (
	githubAPIURL = "https://api.github.com/repos/BlakeLiAFK/sss/releases/latest"
	httpTimeout  = 10 * time.Second
)

// CheckForUpdate 检查是否有新版本
func CheckForUpdate() (*VersionCheckResult, error) {
	result := &VersionCheckResult{
		CurrentVersion: config.Version,
		HasUpdate:      false,
	}

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: httpTimeout,
	}

	// 创建请求
	req, err := http.NewRequest("GET", githubAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置 User-Agent (GitHub API 要求)
	req.Header.Set("User-Agent", "SSS-Server/"+config.Version)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode == http.StatusNotFound {
		// 没有发布任何 release
		result.LatestVersion = config.Version
		return result, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回错误: %d", resp.StatusCode)
	}

	// 解析响应
	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 跳过预发布版本和草稿
	if release.Draft || release.Prerelease {
		result.LatestVersion = config.Version
		return result, nil
	}

	// 获取最新版本号 (去掉 v 前缀)
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	result.LatestVersion = latestVersion
	result.ReleaseURL = release.HTMLURL
	result.ReleaseNotes = release.Body
	result.PublishedAt = release.PublishedAt

	// 比较版本号
	result.HasUpdate = compareVersions(config.Version, latestVersion) < 0

	return result, nil
}

// compareVersions 比较两个版本号
// 返回: -1 (v1 < v2), 0 (v1 == v2), 1 (v1 > v2)
func compareVersions(v1, v2 string) int {
	// 去掉 v 前缀
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// 确保两个版本号有相同的部分数
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int

		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return 0
}
