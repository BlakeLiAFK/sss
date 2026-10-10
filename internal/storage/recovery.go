package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrRecoveryConsumed = errors.New("recovery authorization already consumed")

func recoverySetting(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "recovery.consumed." + hex.EncodeToString(sum[:])
}

func (m *MetadataStore) RecoveryConsumed(token string) (bool, error) {
	value, err := m.GetSetting(recoverySetting(token))
	return value != "", err
}

// ResetAdminPassword consumes the durable local marker token in the same
// transaction as the password update. File cleanup is not a security boundary:
// a crash after commit cannot make the remaining marker reusable.
func (m *MetadataStore) ResetAdminPassword(token, password string) error {
	if len(token) != 64 {
		return errors.New("invalid recovery token")
	}
	if _, err := hex.DecodeString(token); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return m.withWriteLock(func() error {
		tx, err := m.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var value string
		err = tx.QueryRow("SELECT value FROM system_settings WHERE key=?", recoverySetting(token)).Scan(&value)
		if err == nil {
			return ErrRecoveryConsumed
		}
		if err != sql.ErrNoRows {
			return err
		}
		now := time.Now().UTC()
		if _, err = tx.Exec("INSERT INTO system_settings(key,value,updated_at) VALUES(?,?,?)", recoverySetting(token), "used", now); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT OR REPLACE INTO system_settings(key,value,updated_at) VALUES(?,?,?)", SettingAuthAdminPasswordHash, string(hash), now); err != nil {
			return err
		}
		return tx.Commit()
	})
}
