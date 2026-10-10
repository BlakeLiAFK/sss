package admin

import (
	"net/http/httptest"
	"os"
	"sss/internal/config"
	"sss/internal/storage"
	"strings"
	"testing"
)

func TestAuditMalformedMaintenanceDoesNotMutate(t *testing.T) {
	h, cleanup := setupAdminTestHandler(t)
	defer cleanup()
	h.filestore.CreateBucket("audit")
	path, _, err := h.filestore.PutObject("audit", "orphan", strings.NewReader("keep"), 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"", "{", `{}`, `{} {}`, `{"max_upload_age":9223372036854775807}`} {
		w := httptest.NewRecorder()
		h.executeGC(w, httptest.NewRequest("POST", "/api/admin/storage/gc", strings.NewReader(body)))
		if w.Code != 400 || !strings.Contains(w.Header().Get("Content-Type"), "json") {
			t.Fatalf("bad response %d %s", w.Code, w.Body.String())
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal("bad request deleted file", err)
		}
	}
	w := httptest.NewRecorder()
	h.repairIntegrity(w, httptest.NewRequest("POST", "/api/admin/storage/integrity", strings.NewReader(`{"repair":false}`)))
	if w.Code != 400 {
		t.Fatal("repair:false executed")
	}
}
func TestAuditInvalidSettingsAreAtomic(t *testing.T) {
	config.NewDefault()
	h, cleanup := setupAdminTestHandler(t)
	defer cleanup()
	old := config.Global.Server.Region
	w := httptest.NewRecorder()
	h.updateSettings(w, httptest.NewRequest("PUT", "/api/admin/settings", strings.NewReader(`{"region":"changed","presign_scheme":"ftp"}`)))
	if w.Code != 400 || config.Global.Server.Region != old {
		t.Fatal("invalid settings changed memory")
	}
	if value, _ := h.metadata.GetSetting(storage.SettingServerRegion); value != "" {
		t.Fatal("invalid settings changed database")
	}
	w = httptest.NewRecorder()
	h.updateGeoStatsConfig(w, httptest.NewRequest("PUT", "/api/admin/geo-stats/config", strings.NewReader(`{"enabled":false,"mode":"invalid"}`)))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	if value, _ := h.metadata.GetSetting(storage.SettingGeoStatsEnabled); value != "" {
		t.Fatal("invalid geo settings committed")
	}
}
func TestAuditPasswordRevokesSessionsPreservesSpaces(t *testing.T) {
	h, cleanup := setupAdminTestHandler(t)
	defer cleanup()
	if err := h.metadata.SetAdminPassword(" OldPassword123 "); err != nil {
		t.Fatal(err)
	}
	token := sessionStore.CreateSession()
	w := httptest.NewRecorder()
	h.handleChangePassword(w, httptest.NewRequest("POST", "/api/admin/settings/password", strings.NewReader(`{"old_password":" OldPassword123 ","new_password":" NewPassword456 "}`)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if sessionStore.ValidateSession(token) {
		t.Fatal("old session survived")
	}
	if !h.metadata.VerifyAdminPassword(" NewPassword456 ") {
		t.Fatal("password trimmed")
	}
}
func TestAuditPublicFlagRequired(t *testing.T) {
	h, cleanup := setupAdminTestHandler(t)
	defer cleanup()
	h.metadata.CreateBucket("audit")
	h.metadata.UpdateBucketPublic("audit", true)
	for _, body := range []string{`{}`, `{"is_public":true,"isPublic":false}`} {
		w := httptest.NewRecorder()
		h.adminSetBucketPublic(w, httptest.NewRequest("PUT", "/", strings.NewReader(body)), "audit")
		if w.Code != 400 {
			t.Fatal("missing/conflicting flag accepted")
		}
		bucket, _ := h.metadata.GetBucket("audit")
		if !bucket.IsPublic {
			t.Fatal("invalid flag mutated bucket")
		}
	}
}
