package storage

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"sync"
	"time"
)

// APIKey API密钥
type APIKey struct {
	AccessKeyID     string    `json:"access_key_id"`
	SecretAccessKey string    `json:"secret_access_key,omitempty"` // 仅创建时返回
	Description     string    `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
	Enabled         bool      `json:"enabled"`
}

// APIKeyPermission API密钥权限
type APIKeyPermission struct {
	AccessKeyID string `json:"access_key_id"`
	BucketName  string `json:"bucket_name"` // "*" 表示所有桶
	CanRead     bool   `json:"can_read"`
	CanWrite    bool   `json:"can_write"`
}

// APIKeyWithPermissions API密钥及其权限
type APIKeyWithPermissions struct {
	APIKey
	Permissions []APIKeyPermission `json:"permissions"`
}

// CachedAPIKey 缓存的API密钥（包含权限）
type CachedAPIKey struct {
	SecretAccessKey string
	Enabled         bool
	Permissions     map[string]*APIKeyPermission // bucket_name -> permission
}

// APIKeyCache API密钥缓存
type APIKeyCache struct {
	mu    sync.RWMutex
	keys  map[string]*CachedAPIKey // access_key_id -> cached key
	store *MetadataStore
}

// NewAPIKeyCache 创建API密钥缓存
func NewAPIKeyCache(store *MetadataStore) *APIKeyCache {
	cache := &APIKeyCache{
		keys:  make(map[string]*CachedAPIKey),
		store: store,
	}
	// 初始化时加载所有API密钥
	cache.Reload()
	return cache
}

// Reload 重新加载所有API密钥到缓存
func (c *APIKeyCache) Reload() error {
	keys, err := c.store.ListAPIKeysWithPermissions()
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 清空并重建缓存
	c.keys = make(map[string]*CachedAPIKey)
	for _, key := range keys {
		cached := &CachedAPIKey{
			SecretAccessKey: key.SecretAccessKey,
			Enabled:         key.Enabled,
			Permissions:     make(map[string]*APIKeyPermission),
		}
		for i := range key.Permissions {
			perm := key.Permissions[i]
			cached.Permissions[perm.BucketName] = &perm
		}
		c.keys[key.AccessKeyID] = cached
	}
	return nil
}

// Validate 验证API密钥
func (c *APIKeyCache) Validate(accessKeyID, secretAccessKey string) bool {
	c.mu.RLock()
	cached, exists := c.keys[accessKeyID]
	c.mu.RUnlock()

	if !exists || !cached.Enabled {
		return false
	}

	// 使用常量时间比较防止时序攻击
	return subtle.ConstantTimeCompare([]byte(cached.SecretAccessKey), []byte(secretAccessKey)) == 1
}

// GetSecretKey 获取API密钥的SecretKey（用于签名验证）
func (c *APIKeyCache) GetSecretKey(accessKeyID string) (string, bool) {
	c.mu.RLock()
	cached, exists := c.keys[accessKeyID]
	c.mu.RUnlock()

	if !exists || !cached.Enabled {
		return "", false
	}
	return cached.SecretAccessKey, true
}

// CheckPermission 检查API密钥的桶权限
func (c *APIKeyCache) CheckPermission(accessKeyID, bucketName string, needWrite bool) bool {
	c.mu.RLock()
	cached, exists := c.keys[accessKeyID]
	c.mu.RUnlock()

	if !exists || !cached.Enabled {
		return false
	}

	// 先检查通配符权限
	if perm, ok := cached.Permissions["*"]; ok {
		if needWrite {
			return perm.CanWrite
		}
		return perm.CanRead
	}

	// 检查特定桶权限
	perm, ok := cached.Permissions[bucketName]
	if !ok {
		return false
	}

	if needWrite {
		return perm.CanWrite
	}
	return perm.CanRead
}

// === MetadataStore API Key 操作 ===

// CreateAPIKey 创建API密钥（SecretKey 加密存储）
func (m *MetadataStore) CreateAPIKey(description string) (*APIKey, error) {
	accessKeyID := generateRandomKey(20)
	secretAccessKey := generateRandomKey(40)

	// 加密 SecretKey
	encryptedSecret, err := m.EncryptSecret(secretAccessKey)
	if err != nil {
		return nil, err
	}

	_, err = m.db.Exec(`
		INSERT INTO api_keys (access_key_id, secret_access_key, description, created_at, enabled)
		VALUES (?, ?, ?, ?, 1)`,
		accessKeyID, encryptedSecret, description, time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}

	return &APIKey{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey, // 返回明文给用户
		Description:     description,
		CreatedAt:       time.Now().UTC(),
		Enabled:         true,
	}, nil
}

// GetAPIKey 获取API密钥（不返回SecretKey）
func (m *MetadataStore) GetAPIKey(accessKeyID string) (*APIKey, error) {
	var key APIKey
	err := m.db.QueryRow(`
		SELECT access_key_id, description, created_at, enabled
		FROM api_keys WHERE access_key_id = ?`, accessKeyID,
	).Scan(&key.AccessKeyID, &key.Description, &key.CreatedAt, &key.Enabled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &key, err
}

// ListAPIKeys 列出所有API密钥（不返回SecretKey）
func (m *MetadataStore) ListAPIKeys() ([]APIKey, error) {
	rows, err := m.db.Query(`
		SELECT access_key_id, description, created_at, enabled
		FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var key APIKey
		if err := rows.Scan(&key.AccessKeyID, &key.Description, &key.CreatedAt, &key.Enabled); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// ListAPIKeysWithPermissions 列出所有API密钥及其权限（内部使用，包含SecretKey，自动解密）
func (m *MetadataStore) ListAPIKeysWithPermissions() ([]APIKeyWithPermissions, error) {
	rows, err := m.db.Query(`
		SELECT access_key_id, secret_access_key, description, created_at, enabled
		FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKeyWithPermissions
	for rows.Next() {
		var key APIKeyWithPermissions
		var encryptedSecret string
		if err := rows.Scan(&key.AccessKeyID, &encryptedSecret, &key.Description, &key.CreatedAt, &key.Enabled); err != nil {
			return nil, err
		}
		// 解密 SecretKey
		key.SecretAccessKey, err = m.DecryptSecret(encryptedSecret)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	rows.Close()

	// 获取每个密钥的权限
	for i := range keys {
		perms, err := m.GetAPIKeyPermissions(keys[i].AccessKeyID)
		if err != nil {
			return nil, err
		}
		keys[i].Permissions = perms
	}

	return keys, nil
}

// DeleteAPIKey 删除API密钥
func (m *MetadataStore) DeleteAPIKey(accessKeyID string) error {
	_, err := m.db.Exec("DELETE FROM api_keys WHERE access_key_id = ?", accessKeyID)
	return err
}

// UpdateAPIKeyEnabled 启用/禁用API密钥
func (m *MetadataStore) UpdateAPIKeyEnabled(accessKeyID string, enabled bool) error {
	_, err := m.db.Exec("UPDATE api_keys SET enabled = ? WHERE access_key_id = ?", enabled, accessKeyID)
	return err
}

// UpdateAPIKeyDescription 更新API密钥描述
func (m *MetadataStore) UpdateAPIKeyDescription(accessKeyID, description string) error {
	_, err := m.db.Exec("UPDATE api_keys SET description = ? WHERE access_key_id = ?", description, accessKeyID)
	return err
}

// ResetAPIKeySecret 重置API密钥的SecretKey（加密存储）
func (m *MetadataStore) ResetAPIKeySecret(accessKeyID string) (string, error) {
	newSecret := generateRandomKey(40)

	// 加密 SecretKey
	encryptedSecret, err := m.EncryptSecret(newSecret)
	if err != nil {
		return "", err
	}

	result, err := m.db.Exec("UPDATE api_keys SET secret_access_key = ? WHERE access_key_id = ?", encryptedSecret, accessKeyID)
	if err != nil {
		return "", err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return "", sql.ErrNoRows
	}
	return newSecret, nil // 返回明文给用户
}

// === API Key Permission 操作 ===

// SetAPIKeyPermission 设置API密钥的桶权限
func (m *MetadataStore) SetAPIKeyPermission(perm *APIKeyPermission) error {
	_, err := m.db.Exec(`
		INSERT OR REPLACE INTO api_key_permissions (access_key_id, bucket_name, can_read, can_write)
		VALUES (?, ?, ?, ?)`,
		perm.AccessKeyID, perm.BucketName, perm.CanRead, perm.CanWrite,
	)
	return err
}

// DeleteAPIKeyPermission 删除API密钥的桶权限
func (m *MetadataStore) DeleteAPIKeyPermission(accessKeyID, bucketName string) error {
	_, err := m.db.Exec(
		"DELETE FROM api_key_permissions WHERE access_key_id = ? AND bucket_name = ?",
		accessKeyID, bucketName,
	)
	return err
}

// GetAPIKeyPermissions 获取API密钥的所有权限
func (m *MetadataStore) GetAPIKeyPermissions(accessKeyID string) ([]APIKeyPermission, error) {
	rows, err := m.db.Query(`
		SELECT access_key_id, bucket_name, can_read, can_write
		FROM api_key_permissions WHERE access_key_id = ?`, accessKeyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []APIKeyPermission
	for rows.Next() {
		var perm APIKeyPermission
		if err := rows.Scan(&perm.AccessKeyID, &perm.BucketName, &perm.CanRead, &perm.CanWrite); err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return perms, nil
}

// generateRandomKey 生成随机密钥
func generateRandomKey(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand 不可用是严重错误，应立即终止
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}
