package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

// setupAdminTestHandler 创建测试用的管理后台处理器
func setupAdminTestHandler(t *testing.T) (*Handler, func()) {
	t.Helper()

	// 确保配置已初始化
	if config.Global == nil {
		config.NewDefault()
	}

	// 确保日志已初始化
	if utils.Logger == nil {
		utils.InitLogger("info")
	}

	tempDir := t.TempDir()

	// 创建元数据存储
	metadata, err := storage.NewMetadataStore(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("创建元数据存储失败: %v", err)
	}

	// 创建文件存储
	filestore, err := storage.NewFileStore(tempDir)
	if err != nil {
		metadata.Close()
		t.Fatalf("创建文件存储失败: %v", err)
	}

	handler := NewHandler(metadata, filestore)

	cleanup := func() {
		metadata.Close()
	}

	return handler, cleanup
}

// setupInstalledSystem 设置已安装的系统
func setupInstalledSystem(t *testing.T, handler *Handler) string {
	t.Helper()

	// 初始化默认设置（模拟安装）
	err := handler.metadata.InitDefaultSettings("admin", "TestPassword123!")
	if err != nil {
		t.Fatalf("初始化设置失败: %v", err)
	}

	// 标记为已安装
	err = handler.metadata.SetInstalled()
	if err != nil {
		t.Fatalf("设置安装状态失败: %v", err)
	}

	// 创建有效会话并返回 token
	token := sessionStore.CreateSession()
	return token
}

// TestNewHandler 测试处理器创建
func TestNewHandler(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	if handler == nil {
		t.Fatal("Handler 创建失败")
	}
	if handler.metadata == nil {
		t.Error("metadata 为 nil")
	}
	if handler.filestore == nil {
		t.Error("filestore 为 nil")
	}
}

// TestServeHTTP_Routing 测试路由分发
func TestServeHTTP_Routing(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	t.Run("setup路径无需认证", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// 应该返回 200，不是 401
		if rec.Code == http.StatusUnauthorized {
			t.Error("setup 路径不应该需要认证")
		}
	})

	t.Run("login路径无需认证", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// 应该不返回 401（可能返回其他错误，但不是未授权）
		if rec.Code == http.StatusUnauthorized {
			t.Error("login 路径不应该需要认证")
		}
	})

	t.Run("其他admin路径需要认证", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("未认证访问应返回 401, 实际 %d", rec.Code)
		}
	})

	t.Run("有效token可访问受保护路由", func(t *testing.T) {
		token := setupInstalledSystem(t, handler)

		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// 不应该返回 401
		if rec.Code == http.StatusUnauthorized {
			t.Error("有效 token 应该能访问受保护路由")
		}
	})
}

// TestHandleSystemStatus 测试系统状态检查
func TestHandleSystemStatus(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	t.Run("未安装状态", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
		rec := httptest.NewRecorder()

		handler.handleSystemStatus(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response SystemStatusResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if response.Installed != false {
			t.Error("未安装系统应该返回 installed=false")
		}
	})

	t.Run("已安装状态", func(t *testing.T) {
		setupInstalledSystem(t, handler)

		req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
		rec := httptest.NewRecorder()

		handler.handleSystemStatus(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response SystemStatusResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if response.Installed != true {
			t.Error("已安装系统应该返回 installed=true")
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/setup/status", nil)
		rec := httptest.NewRecorder()

		handler.handleSystemStatus(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// TestHandleInstall 测试系统安装
func TestHandleInstall(t *testing.T) {
	t.Run("成功安装", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		body := `{"admin_password": "StrongPassword123!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/install", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleInstall(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var response SetupResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if !response.Success {
			t.Error("安装应该成功")
		}
		if response.AccessKeyID == "" {
			t.Error("应该返回 AccessKeyID")
		}
		if response.SecretAccessKey == "" {
			t.Error("应该返回 SecretAccessKey")
		}
	})

	t.Run("重复安装被拒绝", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		// 先安装一次
		setupInstalledSystem(t, handler)

		// 尝试重复安装
		body := `{"admin_password": "AnotherPassword123!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/install", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleInstall(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("密码为空被拒绝", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		body := `{"admin_username": "admin"}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/install", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleInstall(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("弱密码被拒绝", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		body := `{"admin_password": "weak"}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/install", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleInstall(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		req := httptest.NewRequest(http.MethodGet, "/api/setup/install", nil)
		rec := httptest.NewRecorder()

		handler.handleInstall(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// TestSessionStore 测试会话存储
func TestSessionStore(t *testing.T) {
	t.Run("创建和验证会话", func(t *testing.T) {
		token := sessionStore.CreateSession()

		if token == "" {
			t.Fatal("token 不应为空")
		}

		if !sessionStore.ValidateSession(token) {
			t.Error("新创建的会话应该有效")
		}
	})

	t.Run("删除会话", func(t *testing.T) {
		token := sessionStore.CreateSession()

		sessionStore.DeleteSession(token)

		if sessionStore.ValidateSession(token) {
			t.Error("删除后的会话应该无效")
		}
	})

	t.Run("无效token验证失败", func(t *testing.T) {
		if sessionStore.ValidateSession("invalid-token") {
			t.Error("无效 token 不应该通过验证")
		}
	})
}

// TestLoginRateLimiter 测试登录速率限制
func TestLoginRateLimiter(t *testing.T) {
	t.Run("初始状态未锁定", func(t *testing.T) {
		limiter := &LoginRateLimiter{
			attempts: make(map[string]*LoginAttempt),
		}

		blocked, _ := limiter.IsBlocked("192.168.1.1")
		if blocked {
			t.Error("初始状态不应该被锁定")
		}
	})

	t.Run("多次失败后锁定", func(t *testing.T) {
		limiter := &LoginRateLimiter{
			attempts: make(map[string]*LoginAttempt),
		}

		ip := "192.168.1.2"

		// 模拟多次登录失败
		for i := 0; i < maxLoginAttempts; i++ {
			blocked, _ := limiter.RecordFailure(ip)
			if i < maxLoginAttempts-1 && blocked {
				t.Errorf("第 %d 次失败不应该锁定", i+1)
			}
		}

		// 达到限制后应该被锁定
		blocked, remaining := limiter.IsBlocked(ip)
		if !blocked {
			t.Error("达到失败限制后应该被锁定")
		}
		if remaining <= 0 {
			t.Error("锁定时间应该大于0")
		}
	})

	t.Run("成功登录清除记录", func(t *testing.T) {
		limiter := &LoginRateLimiter{
			attempts: make(map[string]*LoginAttempt),
		}

		ip := "192.168.1.3"

		// 记录几次失败
		limiter.RecordFailure(ip)
		limiter.RecordFailure(ip)

		// 成功登录
		limiter.RecordSuccess(ip)

		// 检查记录是否被清除
		blocked, _ := limiter.IsBlocked(ip)
		if blocked {
			t.Error("成功登录后不应该被锁定")
		}
	})

	t.Run("清理过期记录", func(t *testing.T) {
		limiter := &LoginRateLimiter{
			attempts: make(map[string]*LoginAttempt),
		}

		ip := "192.168.1.4"
		limiter.attempts[ip] = &LoginAttempt{
			FailCount: 3,
			LastFail:  time.Now().Add(-2 * time.Hour), // 2小时前
		}

		limiter.Cleanup()

		// 过期记录应该被清理
		if _, exists := limiter.attempts[ip]; exists {
			t.Error("过期记录应该被清理")
		}
	})
}

// TestHandleAdminLogin 测试管理员登录
func TestHandleAdminLogin(t *testing.T) {
	// 清理速率限制器状态
	loginLimiter = &LoginRateLimiter{
		attempts: make(map[string]*LoginAttempt),
	}

	t.Run("系统未安装时登录失败", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		body := `{"username": "admin", "password": "test123"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleAdminLogin(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusForbidden, rec.Code)
		}
	})

	t.Run("成功登录", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		// 安装系统
		handler.metadata.InitDefaultSettings("admin", "TestPassword123!")
		handler.metadata.SetInstalled()

		body := `{"username": "admin", "password": "TestPassword123!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		handler.handleAdminLogin(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var response AdminLoginResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if !response.Success {
			t.Error("登录应该成功")
		}
		if response.Token == "" {
			t.Error("应该返回 token")
		}
	})

	t.Run("密码错误", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		// 安装系统
		handler.metadata.InitDefaultSettings("admin", "TestPassword123!")
		handler.metadata.SetInstalled()

		body := `{"username": "admin", "password": "WrongPassword"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.100.1:12345"
		rec := httptest.NewRecorder()

		handler.handleAdminLogin(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		req := httptest.NewRequest(http.MethodGet, "/api/admin/login", nil)
		rec := httptest.NewRecorder()

		handler.handleAdminLogin(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// TestHandleAdminLogout 测试管理员登出
func TestHandleAdminLogout(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	token := setupInstalledSystem(t, handler)

	t.Run("成功登出", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
		req.Header.Set("X-Admin-Token", token)
		rec := httptest.NewRecorder()

		handler.handleAdminLogout(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		// 验证会话已被删除
		if sessionStore.ValidateSession(token) {
			t.Error("登出后会话应该失效")
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/logout", nil)
		rec := httptest.NewRecorder()

		handler.handleAdminLogout(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// TestCheckAdminAuth 测试认证检查
func TestCheckAdminAuth(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	t.Run("无token返回false", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets", nil)

		if handler.checkAdminAuth(req) {
			t.Error("无 token 应该返回 false")
		}
	})

	t.Run("有效token从Header返回true", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets", nil)
		req.Header.Set("X-Admin-Token", token)

		if !handler.checkAdminAuth(req) {
			t.Error("有效 token 应该返回 true")
		}
	})

	t.Run("有效token从Cookie返回true", func(t *testing.T) {
		token := sessionStore.CreateSession()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets", nil)
		req.AddCookie(&http.Cookie{Name: "admin_token", Value: token})

		if !handler.checkAdminAuth(req) {
			t.Error("有效 cookie token 应该返回 true")
		}
	})

	t.Run("无效token返回false", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/buckets", nil)
		req.Header.Set("X-Admin-Token", "invalid-token")

		if handler.checkAdminAuth(req) {
			t.Error("无效 token 应该返回 false")
		}
	})
}

// TestRoute 测试路由分发
func TestRoute(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	setupInstalledSystem(t, handler)

	testCases := []struct {
		name         string
		path         string
		method       string
		expectedCode int
	}{
		// 注意：每个测试用例需要独立 token，因为某些操作（如登出）会使 token 失效
		{"API Keys 列表", "/api/admin/apikeys", http.MethodGet, http.StatusOK},
		{"API Key 详情-不存在", "/api/admin/apikeys/nonexistent", http.MethodGet, http.StatusNotFound},
		{"存储桶列表", "/api/admin/buckets", http.MethodGet, http.StatusOK},
		{"存储桶操作-不存在", "/api/admin/buckets/nonexistent", http.MethodGet, http.StatusNotFound},
		{"存储统计", "/api/admin/stats/overview", http.MethodGet, http.StatusOK},
		{"最近对象", "/api/admin/stats/recent", http.MethodGet, http.StatusOK},
		{"GC操作-获取状态", "/api/admin/storage/gc", http.MethodGet, http.StatusOK},
		{"完整性检查-获取状态", "/api/admin/storage/integrity", http.MethodGet, http.StatusOK},
		{"迁移任务列表", "/api/admin/migrate", http.MethodGet, http.StatusOK},
		{"迁移任务-不存在", "/api/admin/migrate/nonexistent", http.MethodGet, http.StatusNotFound},
		{"审计日志", "/api/admin/audit", http.MethodGet, http.StatusOK},
		{"审计统计", "/api/admin/audit/stats", http.MethodGet, http.StatusOK},
		{"系统设置", "/api/admin/settings", http.MethodGet, http.StatusOK},
		{"修改密码-方法错误", "/api/admin/settings/password", http.MethodGet, http.StatusMethodNotAllowed},
		{"不存在的路由", "/api/admin/nonexistent", http.MethodGet, http.StatusNotFound},
		{"登出路由", "/api/admin/logout", http.MethodPost, http.StatusOK}, // 放最后，因为会使 token 失效
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 每个测试创建新的 session token
			token := sessionStore.CreateSession()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Admin-Token", token)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedCode {
				t.Errorf("%s: 期望状态码 %d, 实际 %d", tc.name, tc.expectedCode, rec.Code)
			}
		})
	}
}

// TestHandleResetPasswordCheck 测试密码重置检测
func TestHandleResetPasswordCheck(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	t.Run("检测重置文件状态", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/setup/reset-password/check", nil)
		rec := httptest.NewRecorder()

		handler.handleResetPasswordCheck(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var response ResetPasswordCheckResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		// 文件应该不存在（测试环境）
		if response.FilePath == "" {
			t.Error("FilePath 不应为空")
		}
		if response.Command == "" {
			t.Error("Command 不应为空")
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/setup/reset-password/check", nil)
		rec := httptest.NewRecorder()

		handler.handleResetPasswordCheck(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

// TestHandleResetPassword 测试密码重置
func TestHandleResetPassword(t *testing.T) {
	t.Run("无重置文件拒绝重置", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		// 确保重置文件不存在
		os.Remove(resetPasswordFile)

		body := `{"new_password": "NewPassword123!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/reset-password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleResetPassword(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusForbidden, rec.Code)
		}
	})

	t.Run("方法限制", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		req := httptest.NewRequest(http.MethodGet, "/api/setup/reset-password", nil)
		rec := httptest.NewRecorder()

		handler.handleResetPassword(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("无效JSON格式", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		// 创建临时重置文件
		tempFile := t.TempDir() + "/reset_password"
		os.WriteFile(tempFile, []byte{}, 0644)
		oldResetFile := resetPasswordFile
		resetPasswordFile = tempFile
		defer func() { resetPasswordFile = oldResetFile }()

		body := `{invalid json}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/reset-password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleResetPassword(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("空密码被拒绝", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		// 创建临时重置文件
		tempFile := t.TempDir() + "/reset_password"
		os.WriteFile(tempFile, []byte{}, 0644)
		oldResetFile := resetPasswordFile
		resetPasswordFile = tempFile
		defer func() { resetPasswordFile = oldResetFile }()

		body := `{"new_password": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/reset-password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleResetPassword(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("弱密码被拒绝", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		// 创建临时重置文件
		tempFile := t.TempDir() + "/reset_password"
		os.WriteFile(tempFile, []byte{}, 0644)
		oldResetFile := resetPasswordFile
		resetPasswordFile = tempFile
		defer func() { resetPasswordFile = oldResetFile }()

		body := `{"new_password": "123"}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/reset-password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleResetPassword(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("状态码错误: 期望 %d, 实际 %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("成功重置密码", func(t *testing.T) {
		handler, cleanup := setupAdminTestHandler(t)
		defer cleanup()

		setupInstalledSystem(t, handler)

		// 创建临时重置文件
		tempFile := t.TempDir() + "/reset_password"
		os.WriteFile(tempFile, []byte{}, 0644)
		oldResetFile := resetPasswordFile
		resetPasswordFile = tempFile
		defer func() { resetPasswordFile = oldResetFile }()

		body := `{"new_password": "NewSecurePassword123!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/reset-password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.handleResetPassword(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("状态码错误: 期望 %d, 实际 %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		// 验证重置文件被删除
		if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
			t.Error("重置文件应该被删除")
		}
	})
}

// TestGenerateSessionToken 测试会话令牌生成
func TestGenerateSessionToken(t *testing.T) {
	t.Run("生成唯一令牌", func(t *testing.T) {
		tokens := make(map[string]bool)
		for i := 0; i < 100; i++ {
			token := generateSessionToken()
			if tokens[token] {
				t.Errorf("生成了重复的 token")
			}
			tokens[token] = true

			// 令牌应该是 64 字符的十六进制字符串
			if len(token) != 64 {
				t.Errorf("token 长度错误: 期望 64, 实际 %d", len(token))
			}
		}
	})
}

// TestHandleSetupAPI 测试 setup API 路由
func TestHandleSetupAPI(t *testing.T) {
	handler, cleanup := setupAdminTestHandler(t)
	defer cleanup()

	testCases := []struct {
		name   string
		path   string
		method string
	}{
		{"状态检查", "/api/setup/status", http.MethodGet},
		{"状态检查-空路径", "/api/setup", http.MethodGet},
		{"安装", "/api/setup/install", http.MethodPost},
		{"密码重置检测", "/api/setup/reset-password/check", http.MethodGet},
		{"密码重置", "/api/setup/reset-password", http.MethodPost},
		{"不存在的路径", "/api/setup/nonexistent", http.MethodGet},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var body *bytes.Reader
			if tc.method == http.MethodPost {
				body = bytes.NewReader([]byte(`{}`))
			} else {
				body = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.handleSetupAPI(rec, req)

			// 只验证不 panic，不验证具体状态码
			if rec.Code == 0 {
				t.Error("响应状态码不应为 0")
			}
		})
	}
}

// BenchmarkSessionStore 基准测试会话存储
func BenchmarkSessionStore(b *testing.B) {
	b.Run("创建会话", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sessionStore.CreateSession()
		}
	})

	b.Run("验证会话", func(b *testing.B) {
		token := sessionStore.CreateSession()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sessionStore.ValidateSession(token)
		}
	})
}

// BenchmarkGenerateSessionToken 基准测试令牌生成
func BenchmarkGenerateSessionToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateSessionToken()
	}
}
