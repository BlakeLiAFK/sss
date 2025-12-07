package admin

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// ============================================================================
// API Key 管理测试
// ============================================================================

func TestHandleAPIKeys(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("创建API密钥", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"description":"test api key"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/apikeys", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeys(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp APIKeyResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if resp.AccessKeyID == "" {
			t.Error("AccessKeyID 不应为空")
		}
		if resp.SecretAccessKey == "" {
			t.Error("SecretAccessKey 不应为空（仅在创建时返回）")
		}
		if resp.Description != "test api key" {
			t.Errorf("描述不匹配: 期望 %q, 实际 %q", "test api key", resp.Description)
		}
	})

	t.Run("列出API密钥", func(t *testing.T) {
		token := sessionStore.CreateSession()
		// 先创建一个密钥
		handler.metadata.CreateAPIKey("list test key")

		req := httptest.NewRequest(http.MethodGet, "/api/admin/apikeys", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeys(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var keys []APIKeyResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if len(keys) == 0 {
			t.Error("应该至少有一个密钥")
		}
	})

	t.Run("无效方法返回405", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPatch, "/api/admin/apikeys", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeys(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestHandleAPIKeyDetail(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试密钥
	key, _ := handler.metadata.CreateAPIKey("detail test key")

	t.Run("获取密钥详情", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/apikeys/"+key.AccessKeyID, nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("更新密钥描述", func(t *testing.T) {
		token := sessionStore.CreateSession()
		newDesc := "updated description"
		body := `{"description":"` + newDesc + `"}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/apikeys/"+key.AccessKeyID, bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		// 验证更新
		var resp APIKeyResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Description != newDesc {
			t.Errorf("描述未更新: 期望 %q, 实际 %q", newDesc, resp.Description)
		}
	})

	t.Run("禁用密钥", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"enabled":false}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/apikeys/"+key.AccessKeyID, bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("重置密钥Secret", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/apikeys/"+key.AccessKeyID+"/reset-secret", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/reset-secret")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp APIKeyResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.SecretAccessKey == "" {
			t.Error("重置后应返回新的 SecretAccessKey")
		}
	})

	t.Run("设置密钥权限", func(t *testing.T) {
		token := sessionStore.CreateSession()
		// 先创建桶
		handler.metadata.CreateBucket("perm-test-bucket")

		body := `{"bucket_name":"perm-test-bucket","can_read":true,"can_write":false}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/apikeys/"+key.AccessKeyID+"/permissions", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/permissions")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("设置通配符权限", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"bucket_name":"*","can_read":true,"can_write":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/apikeys/"+key.AccessKeyID+"/permissions", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/permissions")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("删除密钥权限", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/apikeys/"+key.AccessKeyID+"/permissions?bucket_name=perm-test-bucket", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/permissions")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("删除密钥", func(t *testing.T) {
		token := sessionStore.CreateSession()
		// 创建一个新密钥用于删除
		delKey, _ := handler.metadata.CreateAPIKey("to delete")

		req := httptest.NewRequest(http.MethodDelete, "/api/admin/apikeys/"+delKey.AccessKeyID, nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, delKey.AccessKeyID)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("不存在的密钥返回404", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/apikeys/nonexistent", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, "nonexistent")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})
}

// ============================================================================
// 存储桶管理测试
// ============================================================================

func TestHandleAdminBucketsAPI(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("创建存储桶", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"name":"admin-test-bucket"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketsAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("重复创建桶返回冲突", func(t *testing.T) {
		token := sessionStore.CreateSession()
		handler.metadata.CreateBucket("dup-bucket")

		body := `{"name":"dup-bucket"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketsAPI(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusConflict, rec.Code)
		}
	})

	t.Run("列出存储桶", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketsAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var buckets []AdminBucketInfo
		json.Unmarshal(rec.Body.Bytes(), &buckets)
		if len(buckets) == 0 {
			t.Error("应该至少有一个桶")
		}
	})

	t.Run("桶名包含非法字符被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"name":"bucket/../evil"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketsAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("空桶名被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"name":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketsAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestHandleAdminBucketOps(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试桶
	handler.metadata.CreateBucket("ops-test-bucket")
	handler.filestore.CreateBucket("ops-test-bucket")

	t.Run("获取桶详情", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/ops-test-bucket", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "ops-test-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("更新桶公开状态", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"isPublic":true}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/buckets/ops-test-bucket", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "ops-test-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("设置桶为公开", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"is_public":true}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/buckets/ops-test-bucket/public", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "ops-test-bucket/public")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("获取桶公开状态", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/ops-test-bucket/public", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "ops-test-bucket/public")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("删除空桶", func(t *testing.T) {
		token := sessionStore.CreateSession()
		// 创建一个空桶用于删除
		handler.metadata.CreateBucket("del-empty-bucket")
		handler.filestore.CreateBucket("del-empty-bucket")

		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/del-empty-bucket", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "del-empty-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("不存在的桶返回404", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/nonexistent", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "nonexistent")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("不存在的操作返回404", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/ops-test-bucket/unknown-action", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "ops-test-bucket/unknown-action")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})
}

// ============================================================================
// 对象管理测试
// ============================================================================

func TestAdminObjectsHandler(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试桶和对象
	handler.metadata.CreateBucket("obj-test-bucket")
	handler.filestore.CreateBucket("obj-test-bucket")

	t.Run("列出对象", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/obj-test-bucket/objects", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminObjectsHandler(rec, req, "obj-test-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("列出对象带前缀", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/obj-test-bucket/objects?prefix=docs/", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminObjectsHandler(rec, req, "obj-test-bucket")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})
}

func TestAdminDeleteObject(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试桶和对象
	bucketName := "del-obj-bucket"
	handler.metadata.CreateBucket(bucketName)
	handler.filestore.CreateBucket(bucketName)

	// 创建测试文件
	testContent := []byte("test content for delete")
	storagePath, etag, _ := handler.filestore.PutObject(bucketName, "test-delete.txt", bytes.NewReader(testContent), int64(len(testContent)))
	obj := &storage.Object{
		Bucket:      bucketName,
		Key:         "test-delete.txt",
		Size:        int64(len(testContent)),
		ETag:        etag,
		ContentType: "text/plain",
		StoragePath: storagePath,
	}
	handler.metadata.PutObject(obj)

	t.Run("删除对象成功", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/"+bucketName+"/objects?key=test-delete.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDeleteObject(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("缺少key参数", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/"+bucketName+"/objects", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDeleteObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("路径遍历攻击被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/"+bucketName+"/objects?key=../../../etc/passwd", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDeleteObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("不存在的对象返回404", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/"+bucketName+"/objects?key=nonexistent.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDeleteObject(rec, req, bucketName)

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestAdminUploadObject(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试桶
	bucketName := "upload-test-bucket"
	handler.metadata.CreateBucket(bucketName)
	handler.filestore.CreateBucket(bucketName)

	t.Run("上传文件成功", func(t *testing.T) {
		token := sessionStore.CreateSession()

		// 创建 multipart form
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "test.txt")
		part.Write([]byte("test file content"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/upload?key=uploaded/test.txt", &body)
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		handler.adminUploadObject(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("缺少key参数", func(t *testing.T) {
		token := sessionStore.CreateSession()

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "test.txt")
		part.Write([]byte("test file content"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/upload", &body)
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		handler.adminUploadObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("路径遍历攻击被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "test.txt")
		part.Write([]byte("test file content"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/upload?key=../../../evil.txt", &body)
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		handler.adminUploadObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/upload?key=test.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminUploadObject(rec, req, bucketName)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestAdminDownloadObject(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试桶和对象
	bucketName := "download-test-bucket"
	handler.metadata.CreateBucket(bucketName)
	handler.filestore.CreateBucket(bucketName)

	testContent := []byte("download test content")
	storagePath, etag, _ := handler.filestore.PutObject(bucketName, "download.txt", bytes.NewReader(testContent), int64(len(testContent)))
	obj := &storage.Object{
		Bucket:      bucketName,
		Key:         "download.txt",
		Size:        int64(len(testContent)),
		ETag:        etag,
		ContentType: "text/plain",
		StoragePath: storagePath,
	}
	handler.metadata.PutObject(obj)

	t.Run("下载对象成功", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/download?key=download.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDownloadObject(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		if rec.Header().Get("Content-Disposition") == "" {
			t.Error("Content-Disposition header 不应为空")
		}
	})

	t.Run("缺少key参数", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/download", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDownloadObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("路径遍历攻击被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/download?key=../../../etc/passwd", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDownloadObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/download?key=test.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDownloadObject(rec, req, bucketName)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// ============================================================================
// 批量操作测试
// ============================================================================

func TestBatchDeleteObjects(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试桶和对象
	bucketName := "batch-del-bucket"
	handler.metadata.CreateBucket(bucketName)
	handler.filestore.CreateBucket(bucketName)

	// 创建多个测试文件
	for i := 0; i < 3; i++ {
		key := "file" + string(rune('0'+i)) + ".txt"
		content := []byte("content " + key)
		storagePath, etag, _ := handler.filestore.PutObject(bucketName, key, bytes.NewReader(content), int64(len(content)))
		obj := &storage.Object{
			Bucket:      bucketName,
			Key:         key,
			Size:        int64(len(content)),
			ETag:        etag,
			ContentType: "text/plain",
			StoragePath: storagePath,
		}
		handler.metadata.PutObject(obj)
	}

	t.Run("批量删除成功", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"keys":["file0.txt","file1.txt"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/batch/delete", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.batchDeleteObjects(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var result BatchDeleteResult
		json.Unmarshal(rec.Body.Bytes(), &result)
		if result.DeletedCount != 2 {
			t.Errorf("删除数量错误: 期望 2, 实际 %d", result.DeletedCount)
		}
	})

	t.Run("空keys被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"keys":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/batch/delete", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.batchDeleteObjects(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("路径遍历在批量删除中被过滤", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"keys":["../evil.txt","file2.txt"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/batch/delete", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.batchDeleteObjects(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var result BatchDeleteResult
		json.Unmarshal(rec.Body.Bytes(), &result)
		// ../evil.txt 应该失败
		if result.FailedCount == 0 {
			t.Error("路径遍历攻击应该被记录为失败")
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/batch/delete", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.batchDeleteObjects(rec, req, bucketName)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestBatchDownloadObjects(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试桶和对象
	bucketName := "batch-dl-bucket"
	handler.metadata.CreateBucket(bucketName)
	handler.filestore.CreateBucket(bucketName)

	// 创建测试文件
	for i := 0; i < 2; i++ {
		key := "dl-file" + string(rune('0'+i)) + ".txt"
		content := []byte("download content " + key)
		storagePath, etag, _ := handler.filestore.PutObject(bucketName, key, bytes.NewReader(content), int64(len(content)))
		obj := &storage.Object{
			Bucket:      bucketName,
			Key:         key,
			Size:        int64(len(content)),
			ETag:        etag,
			ContentType: "text/plain",
			StoragePath: storagePath,
		}
		handler.metadata.PutObject(obj)
	}

	t.Run("批量下载成功", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"keys":["dl-file0.txt","dl-file1.txt"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/batch/download", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.batchDownloadObjects(rec, req, bucketName)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		if rec.Header().Get("Content-Type") != "application/zip" {
			t.Errorf("Content-Type 错误: 期望 application/zip, 实际 %s", rec.Header().Get("Content-Type"))
		}
	})

	t.Run("空keys被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"keys":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/batch/download", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.batchDownloadObjects(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/batch/download", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.batchDownloadObjects(rec, req, bucketName)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// ============================================================================
// 审计日志测试
// ============================================================================

func TestHandleAuditLogs(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 写入测试审计日志
	handler.metadata.WriteAuditLog(&storage.AuditLog{
		Action:   storage.AuditActionBucketCreate,
		Actor:    "admin",
		Resource: "test-bucket",
		Success:  true,
	})

	t.Run("查询审计日志", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAuditLogs(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp AuditLogResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Total < 1 {
			t.Error("应该至少有一条审计日志")
		}
	})

	t.Run("按操作类型过滤", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit?action=bucket_create", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAuditLogs(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("按成功状态过滤", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit?success=true", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAuditLogs(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("分页查询", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit?page=1&limit=10", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAuditLogs(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp AuditLogResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Limit != 10 {
			t.Errorf("Limit 错误: 期望 10, 实际 %d", resp.Limit)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/audit", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAuditLogs(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestHandleAuditStats(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("获取审计统计", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/stats", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAuditStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/audit/stats", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAuditStats(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// ============================================================================
// 系统设置测试
// ============================================================================

func TestHandleSettings(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 确保 config.Global 已初始化
	if config.Global == nil {
		config.NewDefault()
	}

	t.Run("获取系统设置", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/settings", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleSettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp SettingsResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.System.Installed != true {
			t.Error("Installed 应该为 true")
		}
	})

	t.Run("更新系统设置", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"region":"ap-northeast-1","cors_origin":"https://example.com"}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleSettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("无效presign_scheme被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"presign_scheme":"ftp"}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleSettings(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/settings", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleSettings(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestHandleChangePassword(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("修改密码成功", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"old_password":"TestPassword123!","new_password":"NewPassword456@"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/password", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleChangePassword(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("旧密码错误", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"old_password":"WrongPassword123!","new_password":"NewPassword789#"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/password", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleChangePassword(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("新密码太弱", func(t *testing.T) {
		token := sessionStore.CreateSession()
		// 使用上一个测试修改后的密码
		body := `{"old_password":"NewPassword456@","new_password":"weak"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/password", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleChangePassword(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("密码为空", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"old_password":"","new_password":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/password", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleChangePassword(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/settings/password", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleChangePassword(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// ============================================================================
// 存储统计测试
// ============================================================================

func TestHandleStorageStats(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 确保 config.Global 和 utils.Logger 已初始化
	if config.Global == nil {
		config.NewDefault()
	}
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	t.Run("获取存储统计", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/stats/overview", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleStorageStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/stats/overview", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleStorageStats(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestHandleRecentObjects(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("获取最近对象", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/stats/recent", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleRecentObjects(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("自定义limit", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/stats/recent?limit=5", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleRecentObjects(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/stats/recent", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleRecentObjects(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// ============================================================================
// 垃圾回收测试
// ============================================================================

func TestHandleGC(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("扫描垃圾（预览）", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/gc", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleGC(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("执行垃圾回收", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"max_upload_age":24,"dry_run":false}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/gc", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleGC(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/gc", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleGC(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// ============================================================================
// 完整性检查测试
// ============================================================================

func TestHandleIntegrity(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("检查完整性", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/integrity", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleIntegrity(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("检查完整性（验证ETag）", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/integrity?verify_etag=true&limit=10", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleIntegrity(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("执行修复", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"verify_etag":false,"limit":100,"issues":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/integrity", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleIntegrity(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/integrity", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleIntegrity(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// ============================================================================
// 辅助函数测试
// ============================================================================

func TestContainsDuplicate(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		currentKey string
		expected   bool
	}{
		{"无重复", []string{"a.txt", "b.txt", "c.txt"}, "a.txt", false},
		{"有重复", []string{"dir1/file.txt", "dir2/file.txt"}, "dir1/file.txt", true},
		{"单个文件", []string{"file.txt"}, "file.txt", false},
		{"嵌套目录同名", []string{"a/b/c.txt", "x/y/c.txt", "z/c.txt"}, "a/b/c.txt", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := containsDuplicate(tc.keys, tc.currentKey)
			if result != tc.expected {
				t.Errorf("期望 %v, 实际 %v", tc.expected, result)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"123", 123},
		{"abc", 0},
		{"12abc", 0},
		{"", 0},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, _ := parseInt(tc.input)
			if result != tc.expected {
				t.Errorf("输入 %q: 期望 %d, 实际 %d", tc.input, tc.expected, result)
			}
		})
	}
}

// ============================================================================
// Audit 辅助方法测试
// ============================================================================

func TestAuditMethod(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	t.Run("记录审计日志-字符串详情", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"

		handler.Audit(req, storage.AuditActionBucketCreate, "admin", "test-bucket", true, "创建桶")

		// 验证日志是否写入
		logs, _, _ := handler.metadata.QueryAuditLogs(&storage.AuditLogQuery{
			Action: storage.AuditActionBucketCreate,
			Limit:  1,
		})
		if len(logs) == 0 {
			t.Error("审计日志未写入")
		}
	})

	t.Run("记录审计日志-Map详情", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		detail := map[string]string{"key": "value"}

		handler.Audit(req, storage.AuditActionObjectUpload, "admin", "test-bucket/test.txt", true, detail)
	})

	t.Run("记录审计日志-nil详情", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)

		handler.Audit(req, storage.AuditActionObjectDelete, "admin", "test-bucket/test.txt", true, nil)
	})
}

// ============================================================================
// 迁移功能测试
// ============================================================================

func TestHandleMigrateAPI(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("列出迁移任务（初始为空）", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/migrate", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["stats"] == nil {
			t.Error("stats 不应为 nil")
		}
	})

	t.Run("创建迁移任务-缺少必填字段", func(t *testing.T) {
		token := sessionStore.CreateSession()
		// 缺少 sourceEndpoint
		body := `{"sourceBucket":"test","targetBucket":"local"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/migrate", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleMigrateAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
		}
	})

	t.Run("创建迁移任务-目标桶不存在", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{
			"sourceEndpoint":"https://s3.amazonaws.com",
			"sourceAccessKey":"AKIAIOSFODNN7EXAMPLE",
			"sourceSecretKey":"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			"sourceBucket":"source-bucket",
			"targetBucket":"nonexistent-bucket"
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/migrate", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleMigrateAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/migrate", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateAPI(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestHandleMigrateJob(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 先创建一个目标桶
	handler.metadata.CreateBucket("migrate-target")
	handler.filestore.CreateBucket("migrate-target")

	t.Run("验证配置-缺少必填字段", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"sourceBucket":"test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/migrate/validate", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, "validate")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &resp)
		// 验证应该失败
		if resp["valid"] == true {
			t.Error("缺少必填字段时验证应该失败")
		}
	})

	t.Run("验证配置-方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/migrate/validate", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, "validate")

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("获取不存在的任务", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/migrate/nonexistent-job-id", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, "nonexistent-job-id")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("空任务ID", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/migrate/", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, "")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestMigrateRequest(t *testing.T) {
	// 测试 MigrateRequest 结构体的 JSON 序列化/反序列化
	t.Run("JSON序列化", func(t *testing.T) {
		req := MigrateRequest{
			SourceEndpoint:  "https://s3.amazonaws.com",
			SourceAccessKey: "AKIAIOSFODNN7EXAMPLE",
			SourceSecretKey: "secret",
			SourceBucket:    "source",
			SourcePrefix:    "prefix/",
			SourceRegion:    "us-east-1",
			TargetBucket:    "target",
			TargetPrefix:    "dest/",
			OverwriteExist:  true,
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}

		var decoded MigrateRequest
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("反序列化失败: %v", err)
		}

		if decoded.SourceEndpoint != req.SourceEndpoint {
			t.Errorf("SourceEndpoint 不匹配")
		}
		if decoded.OverwriteExist != req.OverwriteExist {
			t.Errorf("OverwriteExist 不匹配")
		}
	})
}

// ============================================================================
// 增强测试：低覆盖率函数
// ============================================================================

// TestAdminDeleteBucketEnhanced 增强删除桶测试
func TestAdminDeleteBucketEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("删除非空桶失败", func(t *testing.T) {
		// 创建桶并添加对象
		bucketName := "non-empty-bucket"
		handler.metadata.CreateBucket(bucketName)
		handler.filestore.CreateBucket(bucketName)

		// 添加对象
		content := []byte("test content")
		storagePath, etag, _ := handler.filestore.PutObject(bucketName, "test.txt", bytes.NewReader(content), int64(len(content)))
		obj := &storage.Object{
			Bucket:      bucketName,
			Key:         "test.txt",
			Size:        int64(len(content)),
			ETag:        etag,
			ContentType: "text/plain",
			StoragePath: storagePath,
		}
		handler.metadata.PutObject(obj)

		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/"+bucketName, nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, bucketName)

		if rec.Code != http.StatusConflict {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusConflict, rec.Code)
		}
	})

	t.Run("空桶名被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("桶操作方法限制", func(t *testing.T) {
		handler.metadata.CreateBucket("method-test-bucket")
		handler.filestore.CreateBucket("method-test-bucket")

		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPatch, "/api/admin/buckets/method-test-bucket", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "method-test-bucket")

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// TestAPIKeyPermissionEnhanced 增强API Key权限测试
func TestAPIKeyPermissionEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试密钥
	key, _ := handler.metadata.CreateAPIKey("perm-test")

	t.Run("设置权限-空bucket_name", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"bucket_name":"","can_read":true,"can_write":false}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/apikeys/"+key.AccessKeyID+"/permissions", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/permissions")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("设置权限-桶不存在", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"bucket_name":"nonexistent-bucket","can_read":true,"can_write":false}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/apikeys/"+key.AccessKeyID+"/permissions", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/permissions")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("设置权限-无效JSON", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{invalid json}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/apikeys/"+key.AccessKeyID+"/permissions", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/permissions")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("删除权限-从body获取bucket_name", func(t *testing.T) {
		// 先创建桶和权限
		handler.metadata.CreateBucket("body-perm-bucket")
		perm := &storage.APIKeyPermission{
			AccessKeyID: key.AccessKeyID,
			BucketName:  "body-perm-bucket",
			CanRead:     true,
			CanWrite:    true,
		}
		handler.metadata.SetAPIKeyPermission(perm)

		token := sessionStore.CreateSession()
		body := `{"bucket_name":"body-perm-bucket"}`
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/apikeys/"+key.AccessKeyID+"/permissions", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/permissions")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("删除权限-bucket_name为空", func(t *testing.T) {
		token := sessionStore.CreateSession()
		// 不带query参数，body也没有bucket_name
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/apikeys/"+key.AccessKeyID+"/permissions", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/permissions")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("权限操作-方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPut, "/api/admin/apikeys/"+key.AccessKeyID+"/permissions", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/permissions")

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("reset-secret方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/apikeys/"+key.AccessKeyID+"/reset-secret", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/reset-secret")

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("未知操作返回404", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/apikeys/"+key.AccessKeyID+"/unknown", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID+"/unknown")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("APIKey详情-方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPatch, "/api/admin/apikeys/"+key.AccessKeyID, nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("更新APIKey-无效JSON", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{invalid}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/apikeys/"+key.AccessKeyID, bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeyDetail(rec, req, key.AccessKeyID)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("创建APIKey-无效JSON", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{invalid}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/apikeys", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAPIKeys(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// TestAdminObjectsHandlerEnhanced 增强对象处理器测试
func TestAdminObjectsHandlerEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试桶
	handler.metadata.CreateBucket("obj-enhanced-bucket")
	handler.filestore.CreateBucket("obj-enhanced-bucket")

	t.Run("无效方法被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/obj-enhanced-bucket/objects", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminObjectsHandler(rec, req, "obj-enhanced-bucket")

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("无效分页参数", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/obj-enhanced-bucket/objects?limit=abc&page=xyz", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminObjectsHandler(rec, req, "obj-enhanced-bucket")

		// 应该仍然返回200，使用默认值
		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})
}

// TestUpdateSettingsEnhanced 增强设置更新测试
func TestUpdateSettingsEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	if config.Global == nil {
		config.NewDefault()
	}

	t.Run("更新最大对象大小", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"max_object_size":104857600}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleSettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("更新最大上传大小", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"max_upload_size":52428800}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleSettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("更新presign_scheme为https", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"presign_scheme":"https"}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleSettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("更新空cors_origin使用默认值", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"cors_origin":""}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleSettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("无效JSON被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{invalid json}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleSettings(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("更新多个设置项", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"region":"eu-west-1","max_object_size":209715200,"max_upload_size":104857600,"cors_origin":"https://test.com","presign_scheme":"http"}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleSettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})
}

// TestGCEnhanced 增强GC测试
func TestGCEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("扫描垃圾-自定义过期时间", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/storage/gc?max_upload_age=48", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleGC(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("扫描垃圾-无效过期时间使用默认值", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/storage/gc?max_upload_age=invalid", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleGC(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("执行GC-无body使用默认值", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/storage/gc", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleGC(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("执行GC-无效过期时间使用默认值", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"max_upload_age":0,"dry_run":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/storage/gc", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleGC(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("执行GC-dry_run模式", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"max_upload_age":12,"dry_run":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/storage/gc", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleGC(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})
}

// TestIntegrityEnhanced 增强完整性检查测试
func TestIntegrityEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("修复-无效JSON", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{invalid}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/storage/integrity", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleIntegrity(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("修复-带limit参数", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"verify_etag":true,"limit":50}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/storage/integrity", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleIntegrity(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("检查-自定义limit", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/storage/integrity?limit=50", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleIntegrity(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("检查-无效limit使用默认值", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/storage/integrity?limit=invalid", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleIntegrity(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})
}

// TestBucketPublicEnhanced 增强桶公开状态测试
func TestBucketPublicEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建测试桶
	handler.metadata.CreateBucket("public-test-bucket")
	handler.filestore.CreateBucket("public-test-bucket")

	t.Run("设置桶为私有", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"is_public":false}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/buckets/public-test-bucket/public", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "public-test-bucket/public")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("设置公开状态-无效JSON", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{invalid}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/buckets/public-test-bucket/public", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "public-test-bucket/public")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("设置公开状态-方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/public-test-bucket/public", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "public-test-bucket/public")

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("更新桶设置-无效JSON", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{invalid}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/buckets/public-test-bucket", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketOps(rec, req, "public-test-bucket")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// TestChangePasswordEnhanced 增强密码修改测试
func TestChangePasswordEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("无效JSON", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{invalid}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/password", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleChangePassword(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// TestMigrateValidateEnhanced 增强迁移验证测试
func TestMigrateValidateEnhanced(t *testing.T) {
	storage.ResetMigrateManagerForTest()

	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("验证-缺少源访问密钥", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"sourceEndpoint":"http://localhost:9000","sourceBucket":"test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/migrate/validate", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, "validate")

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["valid"] == true {
			t.Error("缺少访问密钥时验证应该失败")
		}
	})

	t.Run("验证-无效JSON", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{invalid}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/migrate/validate", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, "validate")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// TestCreateBucketEnhanced 增强创建桶测试
func TestCreateBucketEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("创建桶-无效JSON", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{invalid}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketsAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("创建桶-包含斜杠", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"name":"bucket/name"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketsAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("创建桶-包含反斜杠", func(t *testing.T) {
		token := sessionStore.CreateSession()
		body := `{"name":"bucket\\name"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminBucketsAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPatch, "/api/admin/buckets", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminBucketsAPI(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// ============================================================================
// 迁移功能测试
// ============================================================================

// TestMigrateJobCancelDelete 测试取消和删除迁移任务的HTTP处理器
func TestMigrateJobCancelDelete(t *testing.T) {
	// 重置迁移管理器单例，避免与其他测试冲突
	storage.ResetMigrateManagerForTest()

	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	// 创建目标桶
	handler.metadata.CreateBucket("migrate-test-cancel")
	handler.filestore.CreateBucket("migrate-test-cancel")

	// 创建一个迁移任务（返回错误时返回空字符串）
	createMigrateJob := func(t *testing.T) string {
		t.Helper()
		token := sessionStore.CreateSession()
		body := `{
			"sourceEndpoint":"http://localhost:19999",
			"sourceAccessKey":"test",
			"sourceSecretKey":"test",
			"sourceBucket":"source",
			"targetBucket":"migrate-test-cancel"
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/migrate", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleMigrateAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Skipf("跳过测试: 创建迁移任务失败 (可能由于单例冲突): %s", rec.Body.String())
			return ""
		}

		var resp map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["jobId"] == nil {
			t.Skip("跳过测试: 无法获取任务ID")
			return ""
		}
		return resp["jobId"].(string)
	}

	t.Run("取消迁移任务", func(t *testing.T) {
		jobID := createMigrateJob(t)
		if jobID == "" {
			return
		}
		// 等待任务进入运行状态或失败
		time.Sleep(100 * time.Millisecond)

		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/migrate/"+jobID+"/cancel", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, jobID+"/cancel")

		// 任务可能已经失败，所以接受 200 或 400
		if rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
			t.Errorf("取消任务状态码错误: 期望 200 或 400, 实际 %d, body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("取消任务-方法限制", func(t *testing.T) {
		jobID := createMigrateJob(t)
		if jobID == "" {
			return
		}
		time.Sleep(50 * time.Millisecond)

		token := sessionStore.CreateSession()
		// 使用 GET 方法应该失败
		req := httptest.NewRequest(http.MethodGet, "/api/admin/migrate/"+jobID+"/cancel", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, jobID+"/cancel")

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("删除迁移任务", func(t *testing.T) {
		jobID := createMigrateJob(t)
		if jobID == "" {
			return
		}
		// 等待任务失败（无法连接到源S3）
		time.Sleep(200 * time.Millisecond)

		// 先尝试取消，确保任务不是运行中
		token := sessionStore.CreateSession()
		cancelReq := httptest.NewRequest(http.MethodPost, "/api/admin/migrate/"+jobID+"/cancel", nil)
		cancelReq.Header.Set("X-Admin-Token", token)
		cancelRec := httptest.NewRecorder()
		handler.handleMigrateJob(cancelRec, cancelReq, jobID+"/cancel")

		// 删除任务
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/migrate/"+jobID, nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, jobID)

		if rec.Code != http.StatusOK {
			t.Errorf("删除任务状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["success"] != true {
			t.Error("删除任务应该成功")
		}
	})

	t.Run("删除不存在的任务", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/migrate/nonexistent", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, "nonexistent")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("获取任务进度", func(t *testing.T) {
		jobID := createMigrateJob(t)
		if jobID == "" {
			return
		}
		time.Sleep(50 * time.Millisecond)

		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/migrate/"+jobID, nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, jobID)

		if rec.Code != http.StatusOK {
			t.Errorf("获取进度状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var progress map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &progress)
		if progress["jobId"] != jobID {
			t.Errorf("任务ID不匹配: 期望 %s, 实际 %v", jobID, progress["jobId"])
		}
	})

	t.Run("未知子操作", func(t *testing.T) {
		jobID := createMigrateJob(t)
		if jobID == "" {
			return
		}
		time.Sleep(50 * time.Millisecond)

		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/migrate/"+jobID+"/unknown", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleMigrateJob(rec, req, jobID+"/unknown")

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})
}

// TestAdminDeleteObjectEnhanced 增强删除对象测试
func TestAdminDeleteObjectEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	bucketName := "delete-obj-test-bucket"

	// 创建测试桶
	token := sessionStore.CreateSession()
	createBucketReq := httptest.NewRequest(http.MethodPost, "/api/admin/buckets",
		strings.NewReader(`{"name":"`+bucketName+`"}`))
	createBucketReq.Header.Set("X-Admin-Token", token)
	createBucketReq.Header.Set("Content-Type", "application/json")
	createBucketRec := httptest.NewRecorder()
	handler.ServeHTTP(createBucketRec, createBucketReq)

	t.Run("缺少key参数", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/"+bucketName+"/objects", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDeleteObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("路径遍历攻击被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/"+bucketName+"/objects?key=../../../etc/passwd", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDeleteObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("删除不存在的对象", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/buckets/"+bucketName+"/objects?key=nonexistent.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDeleteObject(rec, req, bucketName)

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})
}

// TestAdminUploadObjectEnhanced 增强上传对象测试
func TestAdminUploadObjectEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	bucketName := "upload-obj-test-bucket"

	// 创建测试桶
	token := sessionStore.CreateSession()
	createBucketReq := httptest.NewRequest(http.MethodPost, "/api/admin/buckets",
		strings.NewReader(`{"name":"`+bucketName+`"}`))
	createBucketReq.Header.Set("X-Admin-Token", token)
	createBucketReq.Header.Set("Content-Type", "application/json")
	createBucketRec := httptest.NewRecorder()
	handler.ServeHTTP(createBucketRec, createBucketReq)

	t.Run("方法限制-只允许POST", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/upload?key=test.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminUploadObject(rec, req, bucketName)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("缺少key参数", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/upload", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminUploadObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("路径遍历攻击被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/upload?key=../../../etc/passwd", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminUploadObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("无效的multipart表单", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/upload?key=test.txt",
			strings.NewReader("not a valid multipart form"))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
		rec := httptest.NewRecorder()

		handler.adminUploadObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// TestAdminDownloadObjectEnhanced 增强下载对象测试
func TestAdminDownloadObjectEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	bucketName := "download-obj-test-bucket"

	// 创建测试桶
	token := sessionStore.CreateSession()
	createBucketReq := httptest.NewRequest(http.MethodPost, "/api/admin/buckets",
		strings.NewReader(`{"name":"`+bucketName+`"}`))
	createBucketReq.Header.Set("X-Admin-Token", token)
	createBucketReq.Header.Set("Content-Type", "application/json")
	createBucketRec := httptest.NewRecorder()
	handler.ServeHTTP(createBucketRec, createBucketReq)

	t.Run("方法限制-只允许GET", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/buckets/"+bucketName+"/download?key=test.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDownloadObject(rec, req, bucketName)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("缺少key参数", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/download", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDownloadObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("路径遍历攻击被拒绝", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/download?key=../../../etc/passwd", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDownloadObject(rec, req, bucketName)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("下载不存在的对象", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets/"+bucketName+"/download?key=nonexistent.txt", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.adminDownloadObject(rec, req, bucketName)

		if rec.Code != http.StatusNotFound {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusNotFound, rec.Code)
		}
	})
}

// TestRepairIntegrityEnhanced 增强完整性修复测试
func TestRepairIntegrityEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("带issues参数执行修复", func(t *testing.T) {
		token := sessionStore.CreateSession()
		// 提供自定义问题列表
		body := `{"repair": true, "issues": [{"bucket": "test-bucket", "key": "orphan.txt", "type": "orphan_file"}]}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/integrity",
			strings.NewReader(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.repairIntegrity(rec, req)

		// 即使问题不存在，修复也应该成功（只是不修复任何东西）
		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("空issues自动扫描", func(t *testing.T) {
		token := sessionStore.CreateSession()
		// 提供空的 issues 列表
		body := `{"repair": true, "issues": [], "verify_etag": false, "limit": 100}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/integrity",
			strings.NewReader(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.repairIntegrity(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})
}

// TestRecentObjectsEnhanced 增强最近对象测试
func TestRecentObjectsEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("方法限制-只允许GET", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/stats/recent", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleRecentObjects(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("自定义limit参数", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/stats/recent?limit=5", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleRecentObjects(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("超出范围的limit使用默认值", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/stats/recent?limit=100", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleRecentObjects(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})
}

// TestStorageStatsEnhanced 增强存储统计测试
func TestStorageStatsEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("方法限制-只允许GET", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/stats", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleStorageStats(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("获取存储统计成功", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleStorageStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Errorf("解析响应失败: %v", err)
		}

		// 验证响应包含预期字段
		if _, ok := response["stats"]; !ok {
			t.Error("响应缺少 stats 字段")
		}
		if _, ok := response["disk_usage"]; !ok {
			t.Error("响应缺少 disk_usage 字段")
		}
	})
}

// TestLoginEnhanced 增强登录测试
func TestLoginEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("方法限制-只允许POST", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/login", nil)
		rec := httptest.NewRecorder()

		handler.handleAdminLogin(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("空请求体被拒绝", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
		rec := httptest.NewRecorder()

		handler.handleAdminLogin(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("用户名为空被拒绝", func(t *testing.T) {
		body := `{"username": "", "password": "Test1234!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminLogin(rec, req)

		// 登录验证失败返回 401
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("密码为空被拒绝", func(t *testing.T) {
		body := `{"username": "admin", "password": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminLogin(rec, req)

		// 登录验证失败返回 401
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusUnauthorized, rec.Code)
		}
	})
}

// TestLogoutEnhanced 增强登出测试
func TestLogoutEnhanced(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	t.Run("方法限制-只允许POST", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/logout", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminLogout(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("无token的登出", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
		rec := httptest.NewRecorder()

		handler.handleAdminLogout(rec, req)

		// 即使没有 token，也应该返回成功
		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}
	})
}
