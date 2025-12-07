package storage

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 安全错误定义
var (
	ErrInvalidPath = errors.New("invalid path: path traversal detected")
	ErrInvalidKey  = errors.New("invalid key: contains forbidden characters")
)

// FileStore 文件系统存储
type FileStore struct {
	basePath string
}

// NewFileStore 创建文件存储
func NewFileStore(basePath string) (*FileStore, error) {
	// 获取绝对路径
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, err
	}
	return &FileStore{basePath: absPath}, nil
}

// validateKey 验证key是否安全（防止路径遍历攻击）
func validateKey(key string) error {
	// 禁止空key
	if key == "" {
		return ErrInvalidKey
	}
	// 禁止包含..的路径遍历
	if strings.Contains(key, "..") {
		return ErrInvalidPath
	}
	// 禁止绝对路径
	if strings.HasPrefix(key, "/") || strings.HasPrefix(key, "\\") {
		return ErrInvalidPath
	}
	// 禁止包含空字节
	if strings.Contains(key, "\x00") {
		return ErrInvalidPath
	}
	return nil
}

// validateBucket 验证bucket名称是否安全
func validateBucket(bucket string) error {
	if bucket == "" {
		return ErrInvalidKey
	}
	// 禁止包含..的路径遍历
	if strings.Contains(bucket, "..") {
		return ErrInvalidPath
	}
	// 禁止包含路径分隔符
	if strings.ContainsAny(bucket, "/\\") {
		return ErrInvalidPath
	}
	// 禁止包含空字节
	if strings.Contains(bucket, "\x00") {
		return ErrInvalidPath
	}
	return nil
}

// getPath 获取存储路径（内部使用，已验证安全性）
func (f *FileStore) getPath(bucket, key string) (string, error) {
	// 验证bucket和key的安全性
	if err := validateBucket(bucket); err != nil {
		return "", err
	}
	if err := validateKey(key); err != nil {
		return "", err
	}

	// 使用 key 的 hash 前两位作为子目录，避免单目录文件过多
	h := md5.Sum([]byte(key))
	subdir := hex.EncodeToString(h[:1])
	fullPath := filepath.Join(f.basePath, bucket, subdir, key)

	// 确保最终路径在basePath内（双重验证）
	cleanPath := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanPath, f.basePath) {
		return "", ErrInvalidPath
	}

	return cleanPath, nil
}

// getPartPath 获取分片存储路径
func (f *FileStore) getPartPath(uploadID string, partNumber int) (string, error) {
	// 验证uploadID安全性（只允许十六进制字符）
	for _, c := range uploadID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", ErrInvalidPath
		}
	}
	if partNumber < 1 || partNumber > 10000 {
		return "", ErrInvalidPath
	}
	return filepath.Join(f.basePath, ".multipart", uploadID, fmt.Sprintf("%05d", partNumber)), nil
}

// CreateBucket 创建存储桶目录
func (f *FileStore) CreateBucket(name string) error {
	if err := validateBucket(name); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(f.basePath, name), 0755)
}

// DeleteBucket 删除存储桶目录
func (f *FileStore) DeleteBucket(name string) error {
	if err := validateBucket(name); err != nil {
		return err
	}
	bucketPath := filepath.Join(f.basePath, name)
	// 确保路径在basePath内
	cleanPath := filepath.Clean(bucketPath)
	if !strings.HasPrefix(cleanPath, f.basePath) {
		return ErrInvalidPath
	}
	return os.RemoveAll(cleanPath)
}

// PutObject 存储对象并返回 ETag
func (f *FileStore) PutObject(bucket, key string, reader io.Reader, size int64) (string, string, error) {
	path, err := f.getPath(bucket, key)
	if err != nil {
		return "", "", err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", "", err
	}

	file, err := os.Create(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	// 同时计算 MD5
	hash := md5.New()
	writer := io.MultiWriter(file, hash)

	if _, err := io.Copy(writer, reader); err != nil {
		os.Remove(path)
		return "", "", err
	}

	// 确保数据写入磁盘
	if err := file.Sync(); err != nil {
		os.Remove(path)
		return "", "", err
	}

	etag := hex.EncodeToString(hash.Sum(nil))
	return path, etag, nil
}

// GetObject 获取对象
func (f *FileStore) GetObject(storagePath string) (*os.File, error) {
	// 处理相对路径：如果不是以 basePath 开头，尝试将其转换为绝对路径
	cleanPath := filepath.Clean(storagePath)

	// 如果是相对路径，转换为绝对路径
	if !filepath.IsAbs(cleanPath) {
		// 获取当前工作目录
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		cleanPath = filepath.Join(cwd, cleanPath)
	}

	// 验证路径在basePath内
	if !strings.HasPrefix(cleanPath, f.basePath) {
		return nil, ErrInvalidPath
	}
	return os.Open(cleanPath)
}

// DeleteObject 删除对象
func (f *FileStore) DeleteObject(storagePath string) error {
	// 处理相对路径：如果不是以 basePath 开头，尝试将其转换为绝对路径
	cleanPath := filepath.Clean(storagePath)

	// 如果是相对路径，转换为绝对路径
	if !filepath.IsAbs(cleanPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cleanPath = filepath.Join(cwd, cleanPath)
	}

	// 验证路径在basePath内
	if !strings.HasPrefix(cleanPath, f.basePath) {
		return ErrInvalidPath
	}
	return os.Remove(cleanPath)
}

// CopyObject 复制对象到新位置
func (f *FileStore) CopyObject(srcStoragePath, destBucket, destKey string) (string, string, error) {
	// 处理相对路径：如果不是以 basePath 开头，尝试将其转换为绝对路径
	cleanSrcPath := filepath.Clean(srcStoragePath)

	// 如果是相对路径，转换为绝对路径
	if !filepath.IsAbs(cleanSrcPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", err
		}
		cleanSrcPath = filepath.Join(cwd, cleanSrcPath)
	}

	// 验证源路径在basePath内
	if !strings.HasPrefix(cleanSrcPath, f.basePath) {
		return "", "", ErrInvalidPath
	}

	// 打开源文件
	srcFile, err := os.Open(cleanSrcPath)
	if err != nil {
		return "", "", err
	}
	defer srcFile.Close()

	// 获取目标路径
	destPath, err := f.getPath(destBucket, destKey)
	if err != nil {
		return "", "", err
	}

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return "", "", err
	}

	// 创建目标文件
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", "", err
	}
	defer destFile.Close()

	// 同时计算 MD5
	hash := md5.New()
	writer := io.MultiWriter(destFile, hash)

	if _, err := io.Copy(writer, srcFile); err != nil {
		os.Remove(destPath)
		return "", "", err
	}

	// 确保数据写入磁盘
	if err := destFile.Sync(); err != nil {
		os.Remove(destPath)
		return "", "", err
	}

	etag := hex.EncodeToString(hash.Sum(nil))
	return destPath, etag, nil
}

// PutPart 存储分片
func (f *FileStore) PutPart(uploadID string, partNumber int, reader io.Reader) (string, int64, error) {
	path, err := f.getPartPath(uploadID, partNumber)
	if err != nil {
		return "", 0, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", 0, err
	}

	file, err := os.Create(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hash := md5.New()
	writer := io.MultiWriter(file, hash)

	size, err := io.Copy(writer, reader)
	if err != nil {
		os.Remove(path)
		return "", 0, err
	}

	// 确保数据写入磁盘
	if err := file.Sync(); err != nil {
		os.Remove(path)
		return "", 0, err
	}

	etag := hex.EncodeToString(hash.Sum(nil))
	return etag, size, nil
}

// MergeParts 合并分片
func (f *FileStore) MergeParts(bucket, key, uploadID string, partNumbers []int) (string, int64, error) {
	path, err := f.getPath(bucket, key)
	if err != nil {
		return "", 0, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", 0, err
	}

	outFile, err := os.Create(path)
	if err != nil {
		return "", 0, err
	}
	defer outFile.Close()

	hash := md5.New()
	writer := io.MultiWriter(outFile, hash)
	var totalSize int64

	for _, partNum := range partNumbers {
		partPath, err := f.getPartPath(uploadID, partNum)
		if err != nil {
			return "", 0, err
		}
		partFile, err := os.Open(partPath)
		if err != nil {
			return "", 0, err
		}

		n, err := io.Copy(writer, partFile)
		partFile.Close()
		if err != nil {
			return "", 0, err
		}
		totalSize += n
	}

	// 确保数据写入磁盘
	if err := outFile.Sync(); err != nil {
		return "", 0, err
	}

	// 清理分片目录
	os.RemoveAll(filepath.Join(f.basePath, ".multipart", uploadID))

	etag := hex.EncodeToString(hash.Sum(nil))
	return etag, totalSize, nil
}

// AbortMultipartUpload 清理分片
func (f *FileStore) AbortMultipartUpload(uploadID string) error {
	// 验证uploadID安全性
	for _, c := range uploadID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return ErrInvalidPath
		}
	}
	return os.RemoveAll(filepath.Join(f.basePath, ".multipart", uploadID))
}

// GetStoragePath 获取对象存储路径
func (f *FileStore) GetStoragePath(bucket, key string) string {
	path, err := f.getPath(bucket, key)
	if err != nil {
		return ""
	}
	return path
}
