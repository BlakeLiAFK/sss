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

// TestNewFileStore 测试FileStore构造函数
func TestNewFileStore(t *testing.T) {
	t.Run("正常创建", func(t *testing.T) {
		tempDir := t.TempDir()
		fs, err := NewFileStore(tempDir)
		if err != nil {
			t.Fatalf("创建FileStore失败: %v", err)
		}
		if fs == nil {
			t.Fatal("FileStore不应为nil")
		}
		if fs.basePath == "" {
			t.Error("basePath不应为空")
		}
		// 验证目录已创建
		if _, err := os.Stat(fs.basePath); os.IsNotExist(err) {
			t.Error("basePath目录应该存在")
		}
	})

	t.Run("自动创建不存在的目录", func(t *testing.T) {
		tempDir := t.TempDir()
		nonExistPath := filepath.Join(tempDir, "subdir", "storage")
		fs, err := NewFileStore(nonExistPath)
		if err != nil {
			t.Fatalf("创建FileStore失败: %v", err)
		}
		if _, err := os.Stat(fs.basePath); os.IsNotExist(err) {
			t.Error("应该自动创建不存在的目录")
		}
	})

	t.Run("相对路径转绝对路径", func(t *testing.T) {
		fs, err := NewFileStore("./test-storage")
		if err != nil {
			t.Fatalf("创建FileStore失败: %v", err)
		}
		defer os.RemoveAll(fs.basePath)
		if !filepath.IsAbs(fs.basePath) {
			t.Error("basePath应该是绝对路径")
		}
	})
}

// TestCreateBucket 测试创建存储桶
func TestCreateBucket(t *testing.T) {
	fs, cleanup := setupFileStore(t)
	defer cleanup()

	t.Run("正常创建", func(t *testing.T) {
		err := fs.CreateBucket("test-bucket")
		if err != nil {
			t.Fatalf("创建桶失败: %v", err)
		}
		bucketPath := filepath.Join(fs.basePath, "test-bucket")
		if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
			t.Error("桶目录应该存在")
		}
	})

	t.Run("重复创建同名桶", func(t *testing.T) {
		err := fs.CreateBucket("duplicate-bucket")
		if err != nil {
			t.Fatalf("首次创建桶失败: %v", err)
		}
		err = fs.CreateBucket("duplicate-bucket")
		if err != nil {
			t.Errorf("重复创建桶不应报错（幂等性）: %v", err)
		}
	})

	t.Run("无效桶名", func(t *testing.T) {
		invalidNames := []string{"", "../bucket", "bucket/name", "bucket\x00"}
		for _, name := range invalidNames {
			err := fs.CreateBucket(name)
			if err == nil {
				t.Errorf("无效桶名 %q 应该被拒绝", name)
			}
		}
	})
}

// TestDeleteBucket 测试删除存储桶
func TestDeleteBucket(t *testing.T) {
	fs, cleanup := setupFileStore(t)
	defer cleanup()

	t.Run("正常删除", func(t *testing.T) {
		bucket := "delete-test-bucket"
		err := fs.CreateBucket(bucket)
		if err != nil {
			t.Fatalf("创建桶失败: %v", err)
		}
		err = fs.DeleteBucket(bucket)
		if err != nil {
			t.Fatalf("删除桶失败: %v", err)
		}
		bucketPath := filepath.Join(fs.basePath, bucket)
		if _, err := os.Stat(bucketPath); !os.IsNotExist(err) {
			t.Error("桶目录应该已被删除")
		}
	})

	t.Run("删除不存在的桶", func(t *testing.T) {
		err := fs.DeleteBucket("non-existent-bucket")
		// 应该不报错（幂等性）
		if err != nil && !os.IsNotExist(err) {
			t.Errorf("删除不存在的桶不应报错: %v", err)
		}
	})

	t.Run("无效桶名", func(t *testing.T) {
		invalidNames := []string{"", "../bucket", "bucket/name"}
		for _, name := range invalidNames {
			err := fs.DeleteBucket(name)
			if err != ErrInvalidPath && err != ErrInvalidKey {
				t.Errorf("无效桶名 %q 应该返回错误", name)
			}
		}
	})
}

// TestPutObject 测试上传对象
func TestPutObject(t *testing.T) {
	fs, cleanup := setupFileStore(t)
	defer cleanup()

	bucket := "test-bucket"
	fs.CreateBucket(bucket)

	t.Run("正常上传", func(t *testing.T) {
		content := "Hello, World!"
		reader := strings.NewReader(content)
		path, etag, err := fs.PutObject(bucket, "test.txt", reader, int64(len(content)))
		if err != nil {
			t.Fatalf("上传失败: %v", err)
		}
		if path == "" {
			t.Error("存储路径不应为空")
		}
		if etag == "" {
			t.Error("ETag不应为空")
		}
		// 验证文件内容
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取文件失败: %v", err)
		}
		if string(data) != content {
			t.Errorf("文件内容不匹配: got %q, want %q", string(data), content)
		}
	})

	t.Run("空文件上传", func(t *testing.T) {
		reader := strings.NewReader("")
		path, etag, err := fs.PutObject(bucket, "empty.txt", reader, 0)
		if err != nil {
			t.Fatalf("上传空文件失败: %v", err)
		}
		if path == "" || etag == "" {
			t.Error("空文件也应返回有效的路径和ETag")
		}
	})

	t.Run("大文件上传", func(t *testing.T) {
		// 创建1MB的数据
		data := make([]byte, 1024*1024)
		for i := range data {
			data[i] = byte(i % 256)
		}
		reader := strings.NewReader(string(data))
		path, etag, err := fs.PutObject(bucket, "large.bin", reader, int64(len(data)))
		if err != nil {
			t.Fatalf("上传大文件失败: %v", err)
		}
		// 验证文件大小
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("获取文件信息失败: %v", err)
		}
		if info.Size() != int64(len(data)) {
			t.Errorf("文件大小不匹配: got %d, want %d", info.Size(), len(data))
		}
		if etag == "" {
			t.Error("ETag不应为空")
		}
	})

	t.Run("特殊字符文件名", func(t *testing.T) {
		keys := []string{
			"文件 with spaces.txt",
			"中文文件名.txt",
			"file-with-dashes.txt",
			"file_with_underscores.txt",
			"folder/subfolder/file.txt",
		}
		for _, key := range keys {
			reader := strings.NewReader("test")
			_, _, err := fs.PutObject(bucket, key, reader, 4)
			if err != nil {
				t.Errorf("上传文件 %q 失败: %v", key, err)
			}
		}
	})

	t.Run("无效key", func(t *testing.T) {
		invalidKeys := []string{"", "../../../etc/passwd", "/etc/passwd", "file\x00.txt"}
		for _, key := range invalidKeys {
			reader := strings.NewReader("test")
			_, _, err := fs.PutObject(bucket, key, reader, 4)
			if err == nil {
				t.Errorf("无效key %q 应该被拒绝", key)
			}
		}
	})
}

// TestGetObject 测试获取对象
func TestGetObject(t *testing.T) {
	fs, cleanup := setupFileStore(t)
	defer cleanup()

	bucket := "test-bucket"
	fs.CreateBucket(bucket)

	// 先上传一个文件
	content := "test content"
	reader := strings.NewReader(content)
	path, _, err := fs.PutObject(bucket, "test.txt", reader, int64(len(content)))
	if err != nil {
		t.Fatalf("上传文件失败: %v", err)
	}

	t.Run("正常获取", func(t *testing.T) {
		file, err := fs.GetObject(path)
		if err != nil {
			t.Fatalf("获取对象失败: %v", err)
		}
		defer file.Close()
		data, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatalf("读取文件失败: %v", err)
		}
		if string(data) != content {
			t.Errorf("文件内容不匹配: got %q, want %q", string(data), content)
		}
	})

	t.Run("获取不存在的对象", func(t *testing.T) {
		nonExistPath := filepath.Join(fs.basePath, bucket, "xx", "nonexist.txt")
		_, err := fs.GetObject(nonExistPath)
		if err == nil {
			t.Error("获取不存在的对象应该返回错误")
		}
	})

	t.Run("路径遍历攻击", func(t *testing.T) {
		_, err := fs.GetObject("../../../etc/passwd")
		if err == nil {
			t.Error("路径遍历攻击应该被阻止")
		}
	})
}

// TestDeleteObject 测试删除对象
func TestDeleteObject(t *testing.T) {
	fs, cleanup := setupFileStore(t)
	defer cleanup()

	bucket := "test-bucket"
	fs.CreateBucket(bucket)

	t.Run("正常删除", func(t *testing.T) {
		content := "test"
		reader := strings.NewReader(content)
		path, _, err := fs.PutObject(bucket, "delete-test.txt", reader, int64(len(content)))
		if err != nil {
			t.Fatalf("上传文件失败: %v", err)
		}
		err = fs.DeleteObject(path)
		if err != nil {
			t.Fatalf("删除对象失败: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("文件应该已被删除")
		}
	})

	t.Run("删除不存在的对象", func(t *testing.T) {
		nonExistPath := filepath.Join(fs.basePath, bucket, "xx", "nonexist.txt")
		err := fs.DeleteObject(nonExistPath)
		if err == nil {
			t.Error("删除不存在的对象应该返回错误")
		}
	})

	t.Run("路径遍历攻击", func(t *testing.T) {
		err := fs.DeleteObject("../../../etc/passwd")
		if err == nil {
			t.Error("路径遍历攻击应该被阻止")
		}
	})
}

// TestCopyObject 测试复制对象
func TestCopyObject(t *testing.T) {
	fs, cleanup := setupFileStore(t)
	defer cleanup()

	srcBucket := "src-bucket"
	destBucket := "dest-bucket"
	fs.CreateBucket(srcBucket)
	fs.CreateBucket(destBucket)

	// 上传源文件
	content := "copy test content"
	reader := strings.NewReader(content)
	srcPath, srcETag, err := fs.PutObject(srcBucket, "source.txt", reader, int64(len(content)))
	if err != nil {
		t.Fatalf("上传源文件失败: %v", err)
	}

	t.Run("正常复制", func(t *testing.T) {
		destPath, destETag, err := fs.CopyObject(srcPath, destBucket, "dest.txt")
		if err != nil {
			t.Fatalf("复制对象失败: %v", err)
		}
		if destPath == "" || destETag == "" {
			t.Error("复制后的路径和ETag不应为空")
		}
		// ETag应该相同（内容相同）
		if srcETag != destETag {
			t.Errorf("复制后ETag应该相同: src=%s, dest=%s", srcETag, destETag)
		}
		// 验证文件内容
		data, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("读取目标文件失败: %v", err)
		}
		if string(data) != content {
			t.Errorf("复制后内容不匹配: got %q, want %q", string(data), content)
		}
	})

	t.Run("复制到同一桶", func(t *testing.T) {
		destPath, _, err := fs.CopyObject(srcPath, srcBucket, "copy-in-same-bucket.txt")
		if err != nil {
			t.Fatalf("在同一桶内复制失败: %v", err)
		}
		if destPath == "" {
			t.Error("复制后的路径不应为空")
		}
	})

	t.Run("源文件不存在", func(t *testing.T) {
		nonExistPath := filepath.Join(fs.basePath, srcBucket, "xx", "nonexist.txt")
		_, _, err := fs.CopyObject(nonExistPath, destBucket, "dest.txt")
		if err == nil {
			t.Error("复制不存在的源文件应该返回错误")
		}
	})

	t.Run("无效目标路径", func(t *testing.T) {
		_, _, err := fs.CopyObject(srcPath, destBucket, "../../../etc/passwd")
		if err == nil {
			t.Error("无效目标路径应该被拒绝")
		}
	})
}

// TestMultipartUpload 测试多部分上传完整流程
func TestMultipartUpload(t *testing.T) {
	fs, cleanup := setupFileStore(t)
	defer cleanup()

	bucket := "test-bucket"
	fs.CreateBucket(bucket)
	key := "multipart-test.bin"
	uploadID := "1234567890abcdef1234567890abcdef"

	// 创建3个分片
	parts := []struct {
		number  int
		content string
	}{
		{1, "part 1 content"},
		{2, "part 2 content"},
		{3, "part 3 content"},
	}

	t.Run("上传分片", func(t *testing.T) {
		for _, part := range parts {
			reader := strings.NewReader(part.content)
			etag, size, err := fs.PutPart(uploadID, part.number, reader)
			if err != nil {
				t.Fatalf("上传分片 %d 失败: %v", part.number, err)
			}
			if etag == "" {
				t.Errorf("分片 %d 的ETag不应为空", part.number)
			}
			if size != int64(len(part.content)) {
				t.Errorf("分片 %d 大小不匹配: got %d, want %d", part.number, size, len(part.content))
			}
		}
	})

	t.Run("合并分片", func(t *testing.T) {
		partNumbers := []int{1, 2, 3}
		etag, totalSize, err := fs.MergeParts(bucket, key, uploadID, partNumbers)
		if err != nil {
			t.Fatalf("合并分片失败: %v", err)
		}
		if etag == "" {
			t.Error("合并后的ETag不应为空")
		}
		expectedSize := int64(len("part 1 content") + len("part 2 content") + len("part 3 content"))
		if totalSize != expectedSize {
			t.Errorf("合并后大小不匹配: got %d, want %d", totalSize, expectedSize)
		}
		// 验证合并后的文件
		path, err := fs.getPath(bucket, key)
		if err != nil {
			t.Fatalf("获取路径失败: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取合并文件失败: %v", err)
		}
		expected := "part 1 contentpart 2 contentpart 3 content"
		if string(data) != expected {
			t.Errorf("合并后内容不匹配: got %q, want %q", string(data), expected)
		}
	})
}

// TestAbortMultipartUpload 测试中止多部分上传
func TestAbortMultipartUpload(t *testing.T) {
	fs, cleanup := setupFileStore(t)
	defer cleanup()

	uploadID := "abcdef1234567890abcdef1234567890"

	// 上传一些分片
	for i := 1; i <= 3; i++ {
		reader := strings.NewReader("test data")
		_, _, err := fs.PutPart(uploadID, i, reader)
		if err != nil {
			t.Fatalf("上传分片 %d 失败: %v", i, err)
		}
	}

	t.Run("正常中止", func(t *testing.T) {
		err := fs.AbortMultipartUpload(uploadID)
		if err != nil {
			t.Fatalf("中止上传失败: %v", err)
		}
		// 验证分片目录已删除
		partDir := filepath.Join(fs.basePath, ".multipart", uploadID)
		if _, err := os.Stat(partDir); !os.IsNotExist(err) {
			t.Error("分片目录应该已被删除")
		}
	})

	t.Run("无效uploadID", func(t *testing.T) {
		invalidIDs := []string{"../../../etc", "id with spaces", "id/with/slash"}
		for _, id := range invalidIDs {
			err := fs.AbortMultipartUpload(id)
			if err == nil {
				t.Errorf("无效uploadID %q 应该被拒绝", id)
			}
		}
	})
}

// TestGetPartPath 测试获取分片路径
func TestGetPartPath(t *testing.T) {
	fs, cleanup := setupFileStore(t)
	defer cleanup()

	t.Run("正常路径", func(t *testing.T) {
		uploadID := "1234567890abcdef"
		path, err := fs.getPartPath(uploadID, 1)
		if err != nil {
			t.Fatalf("获取分片路径失败: %v", err)
		}
		if !strings.Contains(path, uploadID) {
			t.Errorf("路径应包含uploadID: %s", path)
		}
		if !strings.Contains(path, "00001") {
			t.Errorf("路径应包含格式化的分片号: %s", path)
		}
	})

	t.Run("无效uploadID", func(t *testing.T) {
		invalidIDs := []string{"invalid-chars!", "with spaces", "with/slash"}
		for _, id := range invalidIDs {
			_, err := fs.getPartPath(id, 1)
			if err == nil {
				t.Errorf("无效uploadID %q 应该被拒绝", id)
			}
		}
	})

	t.Run("无效分片号", func(t *testing.T) {
		uploadID := "1234567890abcdef"
		invalidParts := []int{0, -1, 10001}
		for _, partNum := range invalidParts {
			_, err := fs.getPartPath(uploadID, partNum)
			if err == nil {
				t.Errorf("无效分片号 %d 应该被拒绝", partNum)
			}
		}
	})
}

// TestGetStoragePath 测试获取存储路径
func TestGetStoragePath(t *testing.T) {
	fs, cleanup := setupFileStore(t)
	defer cleanup()

	t.Run("正常获取", func(t *testing.T) {
		path := fs.GetStoragePath("bucket", "key.txt")
		if path == "" {
			t.Error("存储路径不应为空")
		}
		if !strings.HasPrefix(path, fs.basePath) {
			t.Error("路径应该在basePath下")
		}
	})

	t.Run("无效输入返回空字符串", func(t *testing.T) {
		invalidCases := []struct {
			bucket, key string
		}{
			{"", "key.txt"},
			{"bucket", ""},
			{"../bucket", "key.txt"},
			{"bucket", "../key.txt"},
		}
		for _, tc := range invalidCases {
			path := fs.GetStoragePath(tc.bucket, tc.key)
			if path != "" {
				t.Errorf("无效输入应返回空字符串: bucket=%q, key=%q, got=%q", tc.bucket, tc.key, path)
			}
		}
	})
}

// setupFileStore 辅助函数：创建测试用的FileStore
func setupFileStore(t *testing.T) (*FileStore, func()) {
	t.Helper()
	tempDir := t.TempDir()
	fs, err := NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("创建FileStore失败: %v", err)
	}
	return fs, func() {
		// cleanup函数，测试结束时自动清理
	}
}
