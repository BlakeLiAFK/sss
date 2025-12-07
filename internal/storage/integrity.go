package storage

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

// IntegrityIssue 完整性问题
type IntegrityIssue struct {
	Bucket     string `json:"bucket"`
	Key        string `json:"key"`
	IssueType  string `json:"issue_type"`  // missing_file, etag_mismatch, path_mismatch
	Expected   string `json:"expected"`    // 预期值
	Actual     string `json:"actual"`      // 实际值
	Size       int64  `json:"size"`        // 文件大小
	Repairable bool   `json:"repairable"` // 是否可修复
}

// IntegrityResult 完整性检查结果
type IntegrityResult struct {
	TotalChecked   int              `json:"total_checked"`    // 检查的对象总数
	IssuesFound    int              `json:"issues_found"`     // 发现的问题数
	Issues         []IntegrityIssue `json:"issues"`           // 问题列表
	MissingFiles   int              `json:"missing_files"`    // 缺失文件数
	EtagMismatches int              `json:"etag_mismatches"`  // ETag 不匹配数
	PathMismatches int              `json:"path_mismatches"`  // 路径不匹配数
	CheckedAt      time.Time        `json:"checked_at"`       // 检查时间
	Duration       float64          `json:"duration"`         // 检查耗时（秒）
	Repaired       bool             `json:"repaired"`         // 是否已修复
	RepairedCount  int              `json:"repaired_count"`   // 修复数量
}

// CheckIntegrity 检查数据完整性
func CheckIntegrity(filestore *FileStore, metadata *MetadataStore, verifyEtag bool, limit int) (*IntegrityResult, error) {
	startTime := time.Now()
	result := &IntegrityResult{
		Issues:    make([]IntegrityIssue, 0),
		CheckedAt: startTime,
	}

	// 获取所有桶
	buckets, err := metadata.ListBuckets()
	if err != nil {
		return nil, err
	}

	checked := 0
	for _, bucket := range buckets {
		// 获取桶中所有对象
		objects, err := metadata.ListAllObjects(bucket.Name)
		if err != nil {
			continue
		}

		for _, obj := range objects {
			// 检查文件是否存在
			if _, err := os.Stat(obj.StoragePath); os.IsNotExist(err) {
				issue := IntegrityIssue{
					Bucket:     obj.Bucket,
					Key:        obj.Key,
					IssueType:  "missing_file",
					Expected:   obj.StoragePath,
					Actual:     "not found",
					Size:       obj.Size,
					Repairable: true, // 可以删除元数据记录
				}
				result.Issues = append(result.Issues, issue)
				result.MissingFiles++
				result.IssuesFound++
			} else if verifyEtag {
				// 验证 ETag
				actualEtag, err := calculateFileEtag(obj.StoragePath)
				if err == nil && actualEtag != obj.ETag {
					// 去掉引号比较
					expectedEtag := trimQuotes(obj.ETag)
					if actualEtag != expectedEtag {
						issue := IntegrityIssue{
							Bucket:     obj.Bucket,
							Key:        obj.Key,
							IssueType:  "etag_mismatch",
							Expected:   obj.ETag,
							Actual:     actualEtag,
							Size:       obj.Size,
							Repairable: true, // 可以更新 ETag
						}
						result.Issues = append(result.Issues, issue)
						result.EtagMismatches++
						result.IssuesFound++
					}
				}
			}

			checked++
			result.TotalChecked = checked

			// 限制检查数量
			if limit > 0 && checked >= limit {
				break
			}
		}

		if limit > 0 && checked >= limit {
			break
		}
	}

	result.Duration = time.Since(startTime).Seconds()
	return result, nil
}

// RepairIntegrity 修复完整性问题
func RepairIntegrity(filestore *FileStore, metadata *MetadataStore, issues []IntegrityIssue) (*IntegrityResult, error) {
	result := &IntegrityResult{
		Issues:    make([]IntegrityIssue, 0),
		CheckedAt: time.Now(),
	}

	for _, issue := range issues {
		if !issue.Repairable {
			continue
		}

		switch issue.IssueType {
		case "missing_file":
			// 删除元数据记录
			if err := metadata.DeleteObject(issue.Bucket, issue.Key); err == nil {
				result.RepairedCount++
			}
		case "etag_mismatch":
			// 重新计算并更新 ETag
			obj, err := metadata.GetObject(issue.Bucket, issue.Key)
			if err != nil {
				continue
			}
			newEtag, err := calculateFileEtag(obj.StoragePath)
			if err != nil {
				continue
			}
			// 更新 ETag
			if err := metadata.UpdateObjectEtag(issue.Bucket, issue.Key, fmt.Sprintf("\"%s\"", newEtag)); err == nil {
				result.RepairedCount++
			}
		}
	}

	result.Repaired = true
	return result, nil
}

// calculateFileEtag 计算文件的 ETag (MD5)
func calculateFileEtag(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// trimQuotes 去掉字符串两端的引号
func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// UpdateObjectEtag 更新对象的 ETag
func (m *MetadataStore) UpdateObjectEtag(bucket, key, etag string) error {
	_, err := m.db.Exec(`
		UPDATE objects
		SET etag = ?
		WHERE bucket = ? AND key = ?
	`, etag, bucket, key)
	return err
}

