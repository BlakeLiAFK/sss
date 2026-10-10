package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sss/internal/auth"
	"sss/internal/config"
	"sss/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestAuditAnonymousExtensionsThroughMux(t *testing.T) {
	s, cleanup := setupHandlersTestServer(t)
	defer cleanup()
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/presign", `{"bucket":"test","key":"x"}`},
		{"PUT", "/api/bucket/test/public", `{"is_public":true}`},
		{"GET", "/api/bucket/test/public", ""}, {"GET", "/api/bucket/test/search?q=x", ""}, {"GET", "/api/bucket/test/head?key=x", ""},
	} {
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != 403 {
			t.Errorf("%s %s: got %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestAuditMultipartResourceBinding(t *testing.T) {
	s, cleanup := setupHandlersTestServer(t)
	defer cleanup()
	s.metadata.CreateBucket("source")
	s.metadata.CreateBucket("other")
	id := "abcdef123456"
	if err := s.metadata.CreateMultipartUpload(&storage.MultipartUpload{UploadID: id, Bucket: "source", Key: "secret", Initiated: time.Now()}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"PUT", "POST", "GET", "DELETE"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "/other/decoy?uploadId="+id+"&partNumber=1", strings.NewReader("x"))
		switch method {
		case "PUT":
			s.handleUploadPart(w, r, "other", "decoy", id)
		case "POST":
			s.handleCompleteMultipartUpload(w, r, "other", "decoy", id)
		case "GET":
			s.handleListParts(w, r, "other", "decoy", id)
		case "DELETE":
			s.handleAbortMultipartUpload(w, r, "other", "decoy", id)
		}
		if w.Code != 404 {
			t.Errorf("%s got %d", method, w.Code)
		}
	}
	if u, _ := s.metadata.GetMultipartUpload(id); u == nil {
		t.Fatal("foreign upload removed")
	}
}

// Direct handler tests explicitly supply the identity established by middleware.
func authorizedUnitRequest(t *testing.T, s *Server, r *http.Request) *http.Request {
	t.Helper()
	key, err := s.metadata.CreateAPIKey("unit test")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.metadata.SetAPIKeyPermission(&storage.APIKeyPermission{AccessKeyID: key.AccessKeyID, BucketName: "*", CanRead: true, CanWrite: true}); err != nil {
		t.Fatal(err)
	}
	auth.InitAPIKeyCache(s.metadata)
	return r.WithContext(context.WithValue(r.Context(), ContextKeyAccessKeyID, key.AccessKeyID))
}

func auditServer(t *testing.T) (*Server, *storage.APIKey) {
	t.Helper()
	config.NewDefault()
	s, cleanup := setupHandlersTestServer(t)
	t.Cleanup(cleanup)
	key, err := s.metadata.CreateAPIKey("audit regression")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.metadata.SetAPIKeyPermission(&storage.APIKeyPermission{AccessKeyID: key.AccessKeyID, BucketName: "*", CanRead: true, CanWrite: true}); err != nil {
		t.Fatal(err)
	}
	auth.InitAPIKeyCache(s.metadata)
	// Reproduce the duplicated legacy setting without granting it special powers.
	config.Global.Auth.AccessKeyID = key.AccessKeyID
	config.Global.Auth.SecretAccessKey = key.SecretAccessKey
	s.metadata.CreateBucket("audit-bucket")
	s.filestore.CreateBucket("audit-bucket")
	return s, key
}
func auditRequest(s *Server, key *storage.APIKey, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	signRequest(r, key.AccessKeyID, key.SecretAccessKey, "us-east-1", []byte(body))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestAuditSubresourcesNeverMutate(t *testing.T) {
	s, key := auditServer(t)
	if w := auditRequest(s, key, "PUT", "/audit-bucket/key", "original", nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, q := range []string{"tagging", "policy", "acl", "versionId=v1", "uploads", "uploadId="} {
		for _, method := range []string{"PUT", "DELETE"} {
			w := auditRequest(s, key, method, "/audit-bucket/key?"+q, "replacement", nil)
			if w.Code < 400 {
				t.Errorf("%s ?%s accepted", method, q)
			}
			if w := auditRequest(s, key, "GET", "/audit-bucket/key", "", nil); w.Body.String() != "original" {
				t.Fatal("original changed", w.Body.String())
			}
		}
	}
	w := auditRequest(s, key, "DELETE", "/audit-bucket?policy", "", nil)
	if w.Code != 501 {
		t.Fatal(w.Code)
	}
}
func TestAuditDefaultKeyRevocationAndRotation(t *testing.T) {
	s, key := auditServer(t)
	old := *key
	secret, err := s.metadata.ResetAPIKeySecret(key.AccessKeyID)
	if err != nil {
		t.Fatal(err)
	}
	auth.ReloadAPIKeyCache()
	if w := auditRequest(s, &old, "GET", "/audit-bucket", "", nil); w.Code != 403 {
		t.Fatal("old secret still accepted")
	}
	key.SecretAccessKey = secret
	if w := auditRequest(s, key, "GET", "/audit-bucket", "", nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.metadata.UpdateAPIKeyEnabled(key.AccessKeyID, false)
	auth.ReloadAPIKeyCache()
	if w := auditRequest(s, key, "GET", "/audit-bucket", "", nil); w.Code != 403 {
		t.Fatal("disabled key accepted")
	}
	s.metadata.DeleteAPIKey(key.AccessKeyID)
	auth.InitAPIKeyCache(s.metadata)
	if w := auditRequest(s, &old, "GET", "/audit-bucket", "", nil); w.Code != 403 {
		t.Fatal("deleted key resurrected")
	}
}
func TestAuditPayloadAndUnknownLengthLimit(t *testing.T) {
	s, key := auditServer(t)
	auditRequest(s, key, "PUT", "/audit-bucket/key", "old", nil)
	r := httptest.NewRequest("PUT", "/audit-bucket/key", strings.NewReader("bad"))
	signRequest(r, key.AccessKeyID, key.SecretAccessKey, "us-east-1", []byte("new"))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("tampered body accepted", w.Code)
	}
	config.Global.Storage.MaxUploadSize = 3
	r = httptest.NewRequest("PUT", "/audit-bucket/key", strings.NewReader("oversize"))
	r.ContentLength = -1
	signRequest(r, key.AccessKeyID, key.SecretAccessKey, "us-east-1", []byte("oversize"))
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatal("streaming limit not enforced", w.Code)
	}
	if w := auditRequest(s, key, "GET", "/audit-bucket/key", "", nil); w.Body.String() != "old" {
		t.Fatal("failed write damaged old blob")
	}
	r = httptest.NewRequest("PUT", "/audit-bucket/new", strings.NewReader("new"))
	r.ContentLength = -1
	signRequest(r, key.AccessKeyID, key.SecretAccessKey, "us-east-1", []byte("new"))
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	obj, _ := s.metadata.GetObject("audit-bucket", "new")
	if obj.Size != 3 {
		t.Fatal("wrong actual size", obj.Size)
	}
}
func TestAuditConcurrentConditionalCreate(t *testing.T) {
	s, key := auditServer(t)
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			codes <- auditRequest(s, key, "PUT", "/audit-bucket/conditional", "value", map[string]string{"If-None-Match": "*"}).Code
		}()
	}
	a, b := <-codes, <-codes
	if !((a == 200 && b == 412) || (a == 412 && b == 200)) {
		t.Fatalf("got %d, %d", a, b)
	}
}
func TestAuditReadConditionsAndSuffixRange(t *testing.T) {
	s, key := auditServer(t)
	put := auditRequest(s, key, "PUT", "/audit-bucket/key", "0123456789", nil)
	w := auditRequest(s, key, "GET", "/audit-bucket/key", "", map[string]string{"Range": "bytes=-3"})
	if w.Code != 206 || w.Body.String() != "789" {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
	for _, method := range []string{"GET", "HEAD"} {
		w := auditRequest(s, key, method, "/audit-bucket/key", "", map[string]string{"If-None-Match": put.Header().Get("ETag")})
		if w.Code != 304 {
			t.Fatal(method, w.Code)
		}
	}
	for _, value := range []string{"bytes=-0", "bytes=20-", "bytes=bad", "bytes=0-1,2-3"} {
		if w := auditRequest(s, key, "GET", "/audit-bucket/key", "", map[string]string{"Range": value}); w.Code != 416 {
			t.Fatal(value, w.Code)
		}
	}
}
func TestAuditRestrictedCopyAndPresign(t *testing.T) {
	s, root := auditServer(t)
	s.metadata.CreateBucket("private-source")
	auditRequest(s, root, "PUT", "/private-source/key", "secret", nil)
	key, _ := s.metadata.CreateAPIKey("restricted")
	s.metadata.SetAPIKeyPermission(&storage.APIKeyPermission{AccessKeyID: key.AccessKeyID, BucketName: "audit-bucket", CanRead: true, CanWrite: true})
	auth.ReloadAPIKeyCache()
	if w := auditRequest(s, key, "PUT", "/audit-bucket/copied", "", map[string]string{"x-amz-copy-source": "/private-source/key"}); w.Code != 403 {
		t.Fatal("copy leaked source", w.Code)
	}
	body := `{"method":"GET","bucket":"private-source","key":"key"}`
	if w := auditRequest(s, key, "POST", "/api/presign", body, nil); w.Code != 403 {
		t.Fatal("presign elevated identity", w.Code)
	}
	body = `{"method":"PUT","bucket":"audit-bucket","key":"space +%中文","contentType":"text/plain"}`
	w := auditRequest(s, key, "POST", "/api/presign", body, nil)
	var result PresignResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(w.Body.String())
	}
	u, err := url.Parse(result.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u.Query().Get("X-Amz-Credential"), key.AccessKeyID+"/") {
		t.Fatal("signer identity changed")
	}
	req := httptest.NewRequest("PUT", result.URL, strings.NewReader("hello"))
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	s.metadata.DeleteAPIKeyPermission(key.AccessKeyID, "audit-bucket")
	auth.ReloadAPIKeyCache()
	req = httptest.NewRequest("PUT", result.URL, strings.NewReader("hello"))
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("presigned permission revocation ignored")
	}
}
