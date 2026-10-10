package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGCStreamingDetailsDoNotLimitCleanup(t *testing.T) {
	fs, ms, cleanup := setupGCTest(t)
	defer cleanup()
	if err := ms.CreateBucket("audit"); err != nil {
		t.Fatal(err)
	}
	live, etag, err := fs.PutObject("audit", "live", strings.NewReader("live"), 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := ms.PutObject(&Object{Bucket: "audit", Key: "live", StoragePath: live, ETag: etag, Size: 4, LastModified: time.Now()}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(fs.basePath, "orphans")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	count := GCDetailLimit + 137
	for i := 0; i < count; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%08d", i)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// More expired upload records than the response sample limit, plus one live.
	tx, err := ms.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		if _, err := tx.Exec("INSERT INTO multipart_uploads(upload_id,bucket,key,initiated) VALUES(?,?,?,?)", fmt.Sprintf("%032x", i+1), "audit", "key", time.Now().Add(-2*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := ms.CreateMultipartUpload(&MultipartUpload{UploadID: "ffffffff", Bucket: "audit", Key: "active", Initiated: time.Now()}); err != nil {
		t.Fatal(err)
	}
	for _, dry := range []bool{true, false} {
		result, err := RunGC(fs, ms, time.Hour, dry)
		if err != nil {
			t.Fatal(err)
		}
		if result.OrphanCount != count || result.ExpiredCount != count || len(result.OrphanFiles) != GCDetailLimit || len(result.ExpiredUploads) != GCDetailLimit || !result.DetailsTruncated {
			t.Fatalf("incorrect bounded result: %+v", result)
		}
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("empty orphan directory not reclaimed", err)
	}
	result, err := RunGC(fs, ms, time.Hour, true)
	if err != nil || result.OrphanCount != 0 || result.ExpiredCount != 0 {
		t.Fatal("cleanup stopped at sample limit", err)
	}
	if data, err := os.ReadFile(live); err != nil || string(data) != "live" {
		t.Fatal("live object deleted", err)
	}
	if upload, err := ms.GetMultipartUpload("ffffffff"); err != nil || upload == nil {
		t.Fatal("active upload deleted", err)
	}
}
