package storage

import (
	"errors"
	"strings"
	"testing"
)

func TestRecoveryTransactionFailureAndReplay(t *testing.T) {
	path := t.TempDir() + "/recovery.db"
	m, err := NewMetadataStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetAdminPassword("OriginalPass123"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 64)
	if _, err := m.db.Exec(`CREATE TRIGGER fail_reset BEFORE INSERT ON system_settings WHEN NEW.key='auth.admin_password_hash' BEGIN SELECT RAISE(FAIL, 'injected password write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := m.ResetAdminPassword(token, "ReplacementPass123"); err == nil {
		t.Fatal("injected failure not returned")
	}
	if !m.VerifyAdminPassword("OriginalPass123") {
		t.Fatal("failed reset changed old password")
	}
	if used, err := m.RecoveryConsumed(token); err != nil || used {
		t.Fatalf("failed transaction consumed marker: %v %v", used, err)
	}
	if _, err := m.db.Exec("DROP TRIGGER fail_reset"); err != nil {
		t.Fatal(err)
	}
	if err := m.ResetAdminPassword(token, "ReplacementPass123"); err != nil {
		t.Fatal(err)
	}
	m.Close()
	m, err = NewMetadataStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.ResetAdminPassword(token, "AnotherPass123"); !errors.Is(err, ErrRecoveryConsumed) {
		t.Fatalf("reopened store accepted replay: %v", err)
	}
	if !m.VerifyAdminPassword("ReplacementPass123") {
		t.Fatal("replay changed password")
	}
}
