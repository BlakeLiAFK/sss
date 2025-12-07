package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

// 加密相关常量
const (
	SettingEncryptionKey = "system.encryption_key"
)

// 加密相关错误
var (
	ErrCiphertextTooShort = errors.New("密文太短")
	ErrInvalidCiphertext  = errors.New("无效的密文")
)

// getOrCreateEncryptionKey 获取或创建加密密钥
func (m *MetadataStore) getOrCreateEncryptionKey() ([]byte, error) {
	keyHex, err := m.GetSetting(SettingEncryptionKey)
	if err != nil {
		return nil, err
	}

	if keyHex != "" {
		// 已存在，解码返回
		return hex.DecodeString(keyHex)
	}

	// 生成新的 256 位密钥
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}

	// 存储密钥
	if err := m.SetSetting(SettingEncryptionKey, hex.EncodeToString(key)); err != nil {
		return nil, err
	}

	return key, nil
}

// EncryptSecret 使用 AES-GCM 加密敏感数据
func (m *MetadataStore) EncryptSecret(plaintext string) (string, error) {
	key, err := m.getOrCreateEncryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 生成随机 nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)

	// 返回 base64 编码的密文
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret 使用 AES-GCM 解密敏感数据
func (m *MetadataStore) DecryptSecret(ciphertextB64 string) (string, error) {
	// 如果是明文（未加密的旧数据），直接返回
	// 加密数据一定是 base64 编码，长度会更长
	if len(ciphertextB64) < 44 {
		// 可能是未加密的旧数据，直接返回
		return ciphertextB64, nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		// 解码失败，可能是明文旧数据
		return ciphertextB64, nil
	}

	key, err := m.getOrCreateEncryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		// 太短，可能是明文旧数据
		return ciphertextB64, nil
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// 解密失败，可能是明文旧数据
		return ciphertextB64, nil
	}

	return string(plaintext), nil
}
