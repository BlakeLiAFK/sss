package api

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sss/internal/auth"
	"sss/internal/config"
	"sss/internal/storage"
	"strings"
	"sync"
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

func TestAuditMultipartOrderSizeAndPagination(t *testing.T) {
	s, key := auditServer(t)
	id := "abcdef987654"
	s.metadata.CreateMultipartUpload(&storage.MultipartUpload{UploadID: id, Bucket: "audit-bucket", Key: "parts", Initiated: time.Now()})
	for i := 1; i <= 3; i++ {
		s.metadata.PutPart(&storage.Part{UploadID: id, PartNumber: i, Size: 5 << 20, ETag: fmt.Sprint(i), ModifiedAt: time.Now()})
	}
	for _, parts := range []string{`<Part><PartNumber>2</PartNumber><ETag>2</ETag></Part><Part><PartNumber>1</PartNumber><ETag>1</ETag></Part>`, `<Part><PartNumber>1</PartNumber><ETag>1</ETag></Part><Part><PartNumber>1</PartNumber><ETag>1</ETag></Part>`, ""} {
		w := auditRequest(s, key, "POST", "/audit-bucket/parts?uploadId="+id, "<CompleteMultipartUpload>"+parts+"</CompleteMultipartUpload>", nil)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := auditRequest(s, key, "GET", "/audit-bucket/parts?uploadId="+id+"&part-number-marker=1&max-parts=1", "", nil)
	var result ListPartsResult
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsTruncated || len(result.Parts) != 1 || result.Parts[0].PartNumber != 2 || result.NextPartNumberMarker != 2 {
		t.Fatalf("pagination %+v", result)
	}
	s.metadata.PutPart(&storage.Part{UploadID: id, PartNumber: 1, Size: 1, ETag: "1", ModifiedAt: time.Now()})
	body := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>1</ETag></Part><Part><PartNumber>2</PartNumber><ETag>2</ETag></Part></CompleteMultipartUpload>`
	w = auditRequest(s, key, "POST", "/audit-bucket/parts?uploadId="+id, body, nil)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "EntityTooSmall") {
		t.Fatal(w.Code, w.Body.String())
	}
	s.metadata.UpdateBucketPublic("audit-bucket", true)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "/audit-bucket/parts?uploadId="+id, nil))
	if w.Code != 403 {
		t.Fatal("public bucket exposed private multipart state")
	}
}
func TestAuditAdminPresignAndRequestID(t *testing.T) {
	s, _ := auditServer(t)
	if _, err := s.metadata.InstallSystem("admin", "TestPassword123!", "us-east-1"); err != nil {
		t.Fatal(err)
	}
	auth.ReloadAPIKeyCache()
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("POST", "/api/admin/login", strings.NewReader(`{"username":"admin","password":"TestPassword123!"}`)))
	var login struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &login); err != nil || login.Token == "" {
		t.Fatal(w.Code, w.Body.String())
	}
	r := httptest.NewRequest("POST", "/api/admin/presign", strings.NewReader(`{"method":"GET","bucket":"audit-bucket","key":"key"}`))
	r.Header.Set("X-Admin-Token", login.Token)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	var result PresignResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || !strings.Contains(result.URL, "example.com/") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "/audit-bucket/key", nil))
	var e struct {
		RequestID string `xml:"RequestId"`
	}
	xml.Unmarshal(w.Body.Bytes(), &e)
	if e.RequestID == "" || e.RequestID != w.Header().Get("x-amz-request-id") {
		t.Fatal("request ID mismatch")
	}
}

func TestAuditInvalidQueryCannotDeleteObject(t *testing.T) {
	s, key := auditServer(t)
	if w := auditRequest(s, key, "PUT", "/audit-bucket/keep", "original", nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	for _, query := range []string{"partNumber=1", "max-parts=1", "prefix=x"} {
		if w := auditRequest(s, key, "DELETE", "/audit-bucket/keep?"+query, "", nil); w.Code != 400 {
			t.Fatal(query, w.Code)
		}
	}
	if w := auditRequest(s, key, "GET", "/audit-bucket/keep", "", nil); w.Body.String() != "original" {
		t.Fatal("invalid query deleted object")
	}
}

func TestAuditPresignCannotInjectCopySource(t *testing.T) {
	s, key := auditServer(t)
	if err := s.metadata.CreateBucket("private-source"); err != nil {
		t.Fatal(err)
	}
	if w := auditRequest(s, key, "PUT", "/private-source/secret", "private bytes", nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if err := s.metadata.UpdateBucketPublic("audit-bucket", true); err != nil {
		t.Fatal(err)
	}
	signedURL := auth.GeneratePresignedURLForKey("PUT", "audit-bucket", "delegated", &auth.PresignOptions{Host: "example.com", Expires: time.Hour}, key.AccessKeyID)
	for _, header := range []string{"X-Amz-Copy-Source", "X-Amz-Metadata-Directive", "X-Amz-Acl", "X-Amz-Tagging", "X-Amz-Meta-Owner"} {
		r := httptest.NewRequest("PUT", signedURL, strings.NewReader(""))
		r.Header.Set(header, "/private-source/secret")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Errorf("unsigned %s accepted: %d %s", header, w.Code, w.Body.String())
		}
		// Header-authenticated requests must enforce the same boundary.
		r = httptest.NewRequest("PUT", "/audit-bucket/delegated", strings.NewReader(""))
		signRequest(r, key.AccessKeyID, key.SecretAccessKey, "us-east-1", []byte(""))
		r.Header.Set(header, "/private-source/secret")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Errorf("unsigned header-auth %s accepted: %d", header, w.Code)
		}

	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "/audit-bucket/delegated", nil))
	if w.Code != 404 {
		t.Fatalf("delegated URL published private data: %d %q", w.Code, w.Body.String())
	}
	r := httptest.NewRequest("PUT", signedURL, strings.NewReader("ordinary upload"))
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("ordinary presigned upload rejected", w.Code, w.Body.String())
	}
}

type auditSlowBody struct {
	entered chan struct{}
	release chan struct{}
	reader  *strings.Reader
	once    sync.Once
}

func (b *auditSlowBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	return b.reader.Read(p)
}
func (b *auditSlowBody) Close() error { return nil }
func TestAuditSlowRequestDoesNotHoldStorageLock(t *testing.T) {
	for _, path := range []string{"/api/admin/login", "/api/setup/install", "/audit-bucket/slow"} {
		t.Run(path, func(t *testing.T) {
			s, key := auditServer(t)
			if path == "/api/admin/login" {
				if _, err := s.metadata.InstallSystem("admin", "TestPassword123!", "us-east-1"); err != nil {
					t.Fatal(err)
				}
			}
			if w := auditRequest(s, key, "PUT", "/audit-bucket/ready", "ready", nil); w.Code != 200 {
				t.Fatal(w.Code)
			}
			body := &auditSlowBody{entered: make(chan struct{}), release: make(chan struct{}), reader: strings.NewReader("{")}
			method := "POST"
			if path == "/audit-bucket/slow" {
				method = "PUT"
			}
			r := httptest.NewRequest(method, path, body)
			r.ContentLength = -1
			if method == "PUT" {
				signRequest(r, key.AccessKeyID, key.SecretAccessKey, "us-east-1", []byte("{"))
			}
			done := make(chan struct{})
			go func() { s.ServeHTTP(httptest.NewRecorder(), r); close(done) }()
			defer func() { close(body.release); <-done }()
			select {
			case <-body.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("slow body not reached")
			}
			for _, target := range []string{"/api/health", "/audit-bucket/ready"} {
				result := make(chan int, 1)
				go func() { result <- auditRequest(s, key, "GET", target, "", nil).Code }()
				select {
				case code := <-result:
					if code != 200 {
						t.Fatal(target, code)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("slow body blocked", target)
				}
			}
		})
	}
}

func TestAuditCopySourceConditions(t *testing.T) {
	s, key := auditServer(t)
	put := auditRequest(s, key, "PUT", "/audit-bucket/source", "source", nil)
	etag := put.Header().Get("ETag")
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	future := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
	for _, tc := range []struct {
		name    string
		headers map[string]string
		code    int
	}{
		{"match fails", map[string]string{"X-Amz-Copy-Source-If-Match": `"wrong"`}, 412},
		{"none fails", map[string]string{"X-Amz-Copy-Source-If-None-Match": etag}, 412},
		{"modified fails", map[string]string{"X-Amz-Copy-Source-If-Modified-Since": future}, 412},
		{"unmodified fails", map[string]string{"X-Amz-Copy-Source-If-Unmodified-Since": past}, 412},
		{"bad date", map[string]string{"X-Amz-Copy-Source-If-Modified-Since": "nonsense"}, 400},
		{"match wins", map[string]string{"X-Amz-Copy-Source-If-Match": etag, "X-Amz-Copy-Source-If-Unmodified-Since": past}, 200},
		{"none still fails", map[string]string{"X-Amz-Copy-Source-If-None-Match": etag, "X-Amz-Copy-Source-If-Modified-Since": past}, 412},
		{"modified succeeds", map[string]string{"X-Amz-Copy-Source-If-Modified-Since": past}, 200},
		{"unmodified succeeds", map[string]string{"X-Amz-Copy-Source-If-Unmodified-Since": future}, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auditRequest(s, key, "PUT", "/audit-bucket/dest", "old destination", nil)
			tc.headers["X-Amz-Copy-Source"] = "/audit-bucket/source"
			w := auditRequest(s, key, "PUT", "/audit-bucket/dest", "", tc.headers)
			if w.Code != tc.code {
				t.Fatal(w.Code, w.Body.String())
			}
			want := "old destination"
			if tc.code == 200 {
				want = "source"
			}
			if got := auditRequest(s, key, "GET", "/audit-bucket/dest", "", nil).Body.String(); got != want {
				t.Fatal("destination damaged", got)
			}
		})
	}
}

func TestAuditAWSSDKPresignedCopyHeaders(t *testing.T) {
	s, key := auditServer(t)
	auditRequest(s, key, "PUT", "/audit-bucket/source", "source", nil)
	req := httptest.NewRequest("PUT", "http://example.com/audit-bucket/copied?X-Amz-Expires=300", nil)
	req.Header.Set("X-Amz-Copy-Source", "/audit-bucket/source")
	signer := v4.NewSigner(func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true; o.DisableHeaderHoisting = true })
	signedURL, headers, err := signer.PresignHTTP(context.Background(), aws.Credentials{AccessKeyID: key.AccessKeyID, SecretAccessKey: key.SecretAccessKey}, req, "UNSIGNED-PAYLOAD", "s3", "us-east-1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, tamper := range []bool{false, true} {
		r := httptest.NewRequest("PUT", signedURL, nil)
		r.Header = headers.Clone()
		if tamper {
			r.Header.Set("X-Amz-Copy-Source", "/audit-bucket/other")
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		want := 200
		if tamper {
			want = 403
		}
		if w.Code != want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
