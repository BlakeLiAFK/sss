package admin

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRecoveryMarkerRetryReplayAndPrivacy(t *testing.T) {
	h, cleanup := setupAdminTestHandler(t)
	defer cleanup()
	old := resetPasswordFile
	resetPasswordFile = t.TempDir() + "/private-server-directory-marker"
	defer func() { resetPasswordFile = old }()
	check := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.handleResetPasswordCheck(rec, httptest.NewRequest(http.MethodGet, "/api/setup/reset-password/check", nil))
		if strings.Contains(rec.Body.String(), resetPasswordFile) || strings.Contains(rec.Body.String(), "command") || strings.Contains(rec.Body.String(), "file_path") {
			t.Fatal("anonymous filesystem disclosure", rec.Body.String())
		}
		return rec
	}
	reset := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.handleResetPassword(rec, httptest.NewRequest(http.MethodPost, "/api/setup/reset-password", strings.NewReader(`{"new_password":"ReplacementPass123"}`)))
		return rec
	}
	if rec := reset(); rec.Code != 403 || strings.Contains(rec.Body.String(), resetPasswordFile) {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if err := os.WriteFile(resetPasswordFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(check().Body.String(), `"file_exists":true`) {
		t.Fatal("empty legacy marker not available")
	}
	// Persist the nonce as a request would, and retain a copy to model a crash
	// between successful database commit and removal of the filesystem marker.
	token, _, err := readRecoveryMarker(true)
	if err != nil {
		t.Fatal(err)
	}
	if rec := reset(); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if !h.metadata.VerifyAdminPassword("ReplacementPass123") {
		t.Fatal("password not updated")
	}
	if err := os.WriteFile(resetPasswordFile, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(check().Body.String(), `"file_exists":true`) {
		t.Fatal("consumed marker advertised as available")
	}
	if rec := reset(); rec.Code != 403 {
		t.Fatal("consumed marker reusable", rec.Code)
	}
	if err := os.Remove(resetPasswordFile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resetPasswordFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	// A storage failure keeps the newly initialized nonce for a subsequent retry.
	h.metadata.Close()
	if rec := reset(); rec.Code != 500 {
		t.Fatal("expected storage failure", rec.Code)
	}
	if data, err := os.ReadFile(resetPasswordFile); err != nil || len(data) != 64 {
		t.Fatal("failure lost authorization", err)
	}
}

func TestRecoveryRejectsNonRegularMarkers(t *testing.T) {
	old := resetPasswordFile
	resetPasswordFile = t.TempDir() + "/marker"
	defer func() { resetPasswordFile = old }()
	if err := os.Mkdir(resetPasswordFile, 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readRecoveryMarker(true); err == nil {
		t.Fatal("directory accepted")
	}
	if err := os.Remove(resetPasswordFile); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir() + "/target"
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, resetPasswordFile); err != nil {
		t.Skip("symlink creation unavailable", err)
	}
	if _, _, err := readRecoveryMarker(true); err == nil {
		t.Fatal("symlink accepted")
	}
	if data, err := os.ReadFile(target); err != nil || len(data) != 0 {
		t.Fatal("symlink target changed", err)
	}
}
