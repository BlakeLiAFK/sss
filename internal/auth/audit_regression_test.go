package auth

import (
	"fmt"
	"net/http/httptest"
	"sss/internal/config"
	"testing"
	"time"
)

func TestAuditSignedTimestampWindow(t *testing.T) {
	setupTestConfig(t)
	for _, offset := range []time.Duration{-time.Hour, time.Hour, 0} {
		stamp := time.Now().UTC().Add(offset)
		r := httptest.NewRequest("GET", "/bucket/key", nil)
		r.Header.Set("X-Amz-Date", stamp.Format("20060102T150405Z"))
		r.Header.Set("X-Amz-Content-Sha256", unsignedPayload)
		headers := "host;x-amz-content-sha256;x-amz-date"
		date := stamp.Format("20060102")
		sig := calculateSignatureWithSecret(r, date, "us-east-1", headers, config.Global.Auth.SecretAccessKey)
		r.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/us-east-1/s3/aws4_request, SignedHeaders=%s, Signature=%s", config.Global.Auth.AccessKeyID, date, headers, sig))
		if VerifyRequest(r) != (offset == 0) {
			t.Fatal("timestamp window", offset)
		}
	}
}
