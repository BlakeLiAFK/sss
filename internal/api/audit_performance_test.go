package api

import (
	"fmt"
	"sss/internal/auth"
	"sss/internal/storage"
	"sss/internal/utils"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Identical benchmark source is run before and after locking changes. Distinct
// object keys share a bucket; every eighth request reads an existing object.
func BenchmarkAuditConcurrentObjects(b *testing.B) {
	for _, size := range []int{64 << 10, 4 << 20, 64 << 20} {
		b.Run(fmt.Sprintf("bytes_%d", size), func(b *testing.B) {
			s, cleanup := setupBenchmark(b)
			defer cleanup()
			utils.InitLogger("error")
			key, err := s.metadata.CreateAPIKey("benchmark generated")
			if err != nil {
				b.Fatal(err)
			}
			if err := s.metadata.ImportLegacyAPIKey(key.AccessKeyID, key.SecretAccessKey); err != nil {
				b.Fatal(err)
			}
			// Generated managed key permissions, never a production credential.
			if err := s.metadata.SetAPIKeyPermission(&storage.APIKeyPermission{AccessKeyID: key.AccessKeyID, BucketName: "*", CanRead: true, CanWrite: true}); err != nil {
				b.Fatal(err)
			}
			auth.ReloadAPIKeyCache()
			payload := strings.Repeat("x", size)
			var sequence atomic.Int64
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					n := sequence.Add(1)
					method, path, body := "PUT", fmt.Sprintf("/bench-bucket/object-%d", n), payload
					if n%8 == 0 {
						method, path, body = "GET", "/bench-bucket/test.txt", ""
					}
					w := auditRequest(s, key, method, path, body, nil)
					if w.Code != 200 {
						b.Errorf("%s: %d", method, w.Code)
					}
				}
			})
		})
	}
}

func TestAuditDistinctKeyWritesProceedIndependently(t *testing.T) {
	s, key := auditServer(t)
	s.filestore.MutationMu.RLock()
	release := s.filestore.LockObject("audit-bucket", "blocked")
	released := false
	defer func() {
		if !released {
			release()
			s.filestore.MutationMu.RUnlock()
		}
	}()
	blocked := make(chan int, 1)
	go func() { blocked <- auditRequest(s, key, "PUT", "/audit-bucket/blocked", "waiting", nil).Code }()
	other := make(chan int, 1)
	go func() { other <- auditRequest(s, key, "PUT", "/audit-bucket/independent", "ready", nil).Code }()
	select {
	case code := <-other:
		if code != 200 {
			t.Fatal(code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("different key blocked")
	}
	select {
	case code := <-blocked:
		t.Fatal("same key ignored lock", code)
	default:
	}
	release()
	s.filestore.MutationMu.RUnlock()
	released = true
	select {
	case code := <-blocked:
		if code != 200 {
			t.Fatal(code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocked key not released")
	}
}
