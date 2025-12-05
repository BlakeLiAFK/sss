package storage

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileStore 文件系统存储
type FileStore struct {
	basePath string
}

// NewFileStore 创建文件存储
func NewFileStore(basePath string) (*FileStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}
	return &FileStore{basePath: basePath}, nil
}

// 获取存储路径
func (f *FileStore) getPath(bucket, key string) string {
	// 使用 key 的 hash 前两位作为子目录，避免单目录文件过多
	h := md5.Sum([]byte(key))
	subdir := hex.EncodeToString(h[:1])
	return filepath.Join(f.basePath, bucket, subdir, key)
}

// 获取分片存储路径
func (f *FileStore) getPartPath(uploadID string, partNumber int) string {
	return filepath.Join(f.basePath, ".multipart", uploadID, fmt.Sprintf("%05d", partNumber))
}

// CreateBucket 创建存储桶目录
func (f *FileStore) CreateBucket(name string) error {
	return os.MkdirAll(filepath.Join(f.basePath, name), 0755)
}

// DeleteBucket 删除存储桶目录
func (f *FileStore) DeleteBucket(name string) error {
	return os.RemoveAll(filepath.Join(f.basePath, name))
}

// PutObject 存储对象并返回 ETag
func (f *FileStore) PutObject(bucket, key string, reader io.Reader, size int64) (string, string, error) {
	path := f.getPath(bucket, key)

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

	etag := hex.EncodeToString(hash.Sum(nil))
	return path, etag, nil
}

// GetObject 获取对象
func (f *FileStore) GetObject(storagePath string) (*os.File, error) {
	return os.Open(storagePath)
}

// DeleteObject 删除对象
func (f *FileStore) DeleteObject(storagePath string) error {
	return os.Remove(storagePath)
}

// PutPart 存储分片
func (f *FileStore) PutPart(uploadID string, partNumber int, reader io.Reader) (string, int64, error) {
	path := f.getPartPath(uploadID, partNumber)

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

	etag := hex.EncodeToString(hash.Sum(nil))
	return etag, size, nil
}

// MergeParts 合并分片
func (f *FileStore) MergeParts(bucket, key, uploadID string, partNumbers []int) (string, int64, error) {
	path := f.getPath(bucket, key)

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
		partPath := f.getPartPath(uploadID, partNum)
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

	// 清理分片目录
	os.RemoveAll(filepath.Join(f.basePath, ".multipart", uploadID))

	etag := hex.EncodeToString(hash.Sum(nil))
	return etag, totalSize, nil
}

// AbortMultipartUpload 清理分片
func (f *FileStore) AbortMultipartUpload(uploadID string) error {
	return os.RemoveAll(filepath.Join(f.basePath, ".multipart", uploadID))
}

// GetStoragePath 获取对象存储路径
func (f *FileStore) GetStoragePath(bucket, key string) string {
	return f.getPath(bucket, key)
}
