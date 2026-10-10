package storage

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type failingReader struct{}

func (failingReader) Read(p []byte) (int, error) { return 0, errors.New("injected read failure") }
func TestAuditImmutableBlobFailureAndSelfCopy(t *testing.T) {
	fs, ms, cleanup := setupIntegrityTest(t)
	defer cleanup()
	ms.CreateBucket("audit")
	old, etag, err := fs.PutObject("audit", "key", strings.NewReader("original"), 8)
	if err != nil {
		t.Fatal(err)
	}
	obj := &Object{Bucket: "audit", Key: "key", Size: 8, ETag: etag, StoragePath: old, LastModified: time.Now()}
	if err := ms.PutObject(obj); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fs.PutObject("audit", "key", io.MultiReader(strings.NewReader("partial"), failingReader{}), 100); err == nil {
		t.Fatal("expected failure")
	}
	if b, _ := os.ReadFile(old); string(b) != "original" {
		t.Fatal("old blob damaged")
	}
	copied, _, err := fs.CopyObject(old, "audit", "key")
	if err != nil {
		t.Fatal(err)
	}
	if copied == old {
		t.Fatal("copy reused immutable path")
	}
	if b, _ := os.ReadFile(copied); string(b) != "original" {
		t.Fatal("self copy truncated")
	}
	if _, err := ms.db.Exec("CREATE TRIGGER fail_commit BEFORE INSERT ON objects BEGIN SELECT RAISE(FAIL,'injected commit failure'); END"); err != nil {
		t.Fatal(err)
	}
	obj.StoragePath = copied
	if err := ms.PutObject(obj); err == nil {
		t.Fatal("expected metadata failure")
	}
	fs.DeleteObject(copied)
	current, err := ms.GetObject("audit", "key")
	if err != nil || current.StoragePath != old {
		t.Fatal("pointer changed after failed commit")
	}
	if b, _ := os.ReadFile(old); string(b) != "original" {
		t.Fatal("rollback deleted old data")
	}
}
func TestAuditLogicalDirectoryPagination(t *testing.T) {
	ms, cleanup := setupMetadataStore(t)
	defer cleanup()
	ms.CreateBucket("audit")
	for _, key := range []string{"p/a/1", "p/a/2", "p/b/1", "a_1", "ab1", "100%/x"} {
		if err := ms.PutObject(&Object{Bucket: "audit", Key: key, LastModified: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := ms.ListObjects("audit", "p/", "", "/", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !first.IsTruncated || first.NextMarker != "p/a/" || len(first.CommonPrefixes) != 1 {
		t.Fatalf("first: %+v", first)
	}
	second, _ := ms.ListObjects("audit", "p/", first.NextMarker, "/", 1)
	if second.IsTruncated || second.NextMarker != "p/b/" {
		t.Fatalf("second: %+v", second)
	}
	literal, _ := ms.ListObjects("audit", "a_", "", "", 100)
	if len(literal.Contents) != 1 || literal.Contents[0].Key != "a_1" {
		t.Fatal("LIKE wildcard leaked")
	}
	root, _ := ms.ListObjects("audit", "", "", "/", 100)
	if len(root.CommonPrefixes) != 2 {
		t.Fatalf("root delimiter %+v", root)
	}
}
func TestAuditMigrationCancellationAndSnapshots(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()
	store.CreateBucket("target")
	started, cancelled := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(cancelled) }))
	defer server.Close()
	key, err := store.CreateAPIKey("generated fixture")
	if err != nil {
		t.Fatal(err)
	}
	id, err := manager.StartMigration(MigrateConfig{SourceEndpoint: server.URL, SourceAccessKey: key.AccessKeyID, SourceSecretKey: key.SecretAccessKey, SourceBucket: "source", TargetBucket: "target"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("migration did not begin")
	}
	snapshot := manager.GetProgress(id)
	data, _ := json.Marshal(snapshot)
	if strings.Contains(string(data), key.SecretAccessKey) {
		t.Fatal("secret exposed")
	}
	snapshot.Status = "corrupted"
	if manager.GetProgress(id).Status == "corrupted" {
		t.Fatal("mutable snapshot")
	}
	if err := manager.CancelMigration(id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("I/O was not cancelled")
	}
	if manager.GetProgress(id).Status != "cancelled" {
		t.Fatal("cancel overwritten")
	}
}
