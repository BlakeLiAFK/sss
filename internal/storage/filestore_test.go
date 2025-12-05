package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateKey 测试密钥验证函数
func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"正常密钥", "test.txt", false},
		{"带路径的密钥", "folder/subfolder/file.txt", false},
		{"空密钥", "", true},
		{"路径遍历攻击1", "../../../etc/passwd", true},
		{"路径遍历攻击2", "folder/../../../etc/passwd", true},
		{"路径遍历攻击3", "..\\..\\..\\windows\\system32", true},
		{"以斜杠开头", "/etc/passwd", true},
		{"以反斜杠开头", "\\windows\\system32", true},
		{"包含空字符", "test\x00.txt", true},
		{"中文文件名", "测试文件.txt", false},
		{"带空格的文件名", "test file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

// TestValidateBucket 测试存储桶名称验证函数
func TestValidateBucket(t *testing.T) {
	tests := []struct {
		name    string
		bucket  string
		wantErr bool
	}{
		{"正常桶名", "my-bucket", false},
		{"带数字的桶名", "bucket123", false},
		{"空桶名", "", true},
		{"路径遍历攻击", "../other-bucket", true},
		{"包含斜杠", "bucket/name", true},
		{"包含反斜杠", "bucket\\name", true},
		{"包含空字符", "bucket\x00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBucket(tt.bucket)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBucket(%q) error = %v, wantErr %v", tt.bucket, err, tt.wantErr)
			}
		})
	}
}

// TestGetPath 测试路径生成函数的安全性
func TestGetPath(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "filestore-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := &FileStore{basePath: tempDir}

	tests := []struct {
		name       string
		bucket     string
		key        string
		wantErr    bool
		wantPrefix bool // 是否需要检查路径前缀
	}{
		{"正常路径", "bucket", "test.txt", false, true},
		{"带子目录的路径", "bucket", "folder/file.txt", false, true},
		{"路径遍历攻击", "bucket", "../../../etc/passwd", true, false},
		{"桶名路径遍历", "../bucket", "file.txt", true, false},
		{"空桶名", "", "file.txt", true, false},
		{"空密钥", "bucket", "", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := fs.getPath(tt.bucket, tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPath(%q, %q) error = %v, wantErr %v", tt.bucket, tt.key, err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.wantPrefix {
				// 验证生成的路径确实在basePath下
				if !strings.HasPrefix(path, tempDir) {
					t.Errorf("getPath(%q, %q) = %q, 不在basePath %q 下", tt.bucket, tt.key, path, tempDir)
				}
			}
		})
	}
}

// TestFileStoreIntegration 集成测试
func TestFileStoreIntegration(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "filestore-integration")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs, err := NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("创建FileStore失败: %v", err)
	}

	// 测试创建桶
	err = fs.CreateBucket("test-bucket")
	if err != nil {
		t.Fatalf("创建桶目录失败: %v", err)
	}

	// 验证桶目录已创建
	bucketPath := filepath.Join(tempDir, "test-bucket")
	if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
		t.Errorf("桶目录未创建: %s", bucketPath)
	}

	// 测试上传对象
	content := strings.NewReader("Hello, World!")
	storagePath, etag, err := fs.PutObject("test-bucket", "hello.txt", content, 13)
	if err != nil {
		t.Fatalf("上传对象失败: %v", err)
	}

	if etag == "" {
		t.Error("ETag不应为空")
	}

	if storagePath == "" {
		t.Error("存储路径不应为空")
	}

	// 测试获取对象
	file, err := fs.GetObject(storagePath)
	if err != nil {
		t.Fatalf("获取对象失败: %v", err)
	}
	defer file.Close()

	// 测试删除对象
	err = fs.DeleteObject(storagePath)
	if err != nil {
		t.Fatalf("删除对象失败: %v", err)
	}

	// 验证文件已删除
	if _, err := os.Stat(storagePath); !os.IsNotExist(err) {
		t.Error("文件应该已被删除")
	}
}

// TestPathTraversalPrevention 专门测试路径遍历防护
func TestPathTraversalPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filestore-security")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := &FileStore{basePath: tempDir}

	// 创建一个"敏感"文件在basePath外
	sensitiveDir := filepath.Dir(tempDir)
	sensitiveFile := filepath.Join(sensitiveDir, "sensitive.txt")
	os.WriteFile(sensitiveFile, []byte("secret"), 0644)
	defer os.Remove(sensitiveFile)

	// 尝试路径遍历攻击
	attacks := []struct {
		bucket string
		key    string
	}{
		{"bucket", "../sensitive.txt"},
		{"bucket", "../../sensitive.txt"},
		{"../", "sensitive.txt"},
		{"bucket", "folder/../../../sensitive.txt"},
	}

	for _, attack := range attacks {
		_, err := fs.getPath(attack.bucket, attack.key)
		if err == nil {
			t.Errorf("路径遍历攻击应该被阻止: bucket=%q, key=%q", attack.bucket, attack.key)
		}
	}
}
