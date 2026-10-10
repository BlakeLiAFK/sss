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

func TestAuditCancelDuringObjectTransferPreservesOldObject(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()
	store.CreateBucket("target")
	old, etag, err := manager.fileStore.PutObject("target", "key", strings.NewReader("old"), 3)
	if err != nil {
		t.Fatal(err)
	}
	store.PutObject(&Object{Bucket: "target", Key: "key", StoragePath: old, Size: 3, ETag: etag, LastModified: time.Now()})
	started, cancelled := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("list-type") == "2" {
			w.Header().Set("Content-Type", "application/xml")
			io.WriteString(w, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>key</Key><Size>4</Size></Contents></ListBucketResult>`)
			return
		}
		w.Header().Set("Content-Length", "4")
		io.WriteString(w, "p")
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer server.Close()
	key, _ := store.CreateAPIKey("generated source credential")
	id, err := manager.StartMigration(MigrateConfig{SourceEndpoint: server.URL, SourceAccessKey: key.AccessKeyID, SourceSecretKey: key.SecretAccessKey, SourceBucket: "source", TargetBucket: "target", OverwriteExist: true})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("transfer did not start")
	}
	if err := manager.CancelMigration(id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("transfer not cancelled")
	}
	manager.fileStore.MutationMu.RLock()
	defer manager.fileStore.MutationMu.RUnlock()
	current, err := store.GetObject("target", "key")
	if err != nil || current.StoragePath != old {
		t.Fatal("cancel published object")
	}
	if bytes, _ := os.ReadFile(old); string(bytes) != "old" {
		t.Fatal("old data changed")
	}
}
func TestAuditMultipartColumnMigration(t *testing.T) {
	path := t.TempDir() + "/legacy.db"
	store, err := NewMetadataStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("ALTER TABLE parts DROP COLUMN storage_path"); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = NewMetadataStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.PutPart(&Part{UploadID: "abc", PartNumber: 1, ModifiedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	parts, err := store.ListParts("abc")
	if err != nil || len(parts) != 1 || parts[0].StoragePath != "" {
		t.Fatal("legacy part migration failed", err)
	}
}

func TestAuditValidateMigrationLocalS3(t *testing.T) {
	manager, store, cleanup := setupMigrateManager(t)
	defer cleanup()
	key, err := store.CreateAPIKey("isolated validation")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("max-keys") != "1" || r.Header.Get("Authorization") == "" {
			t.Error("validation must issue a signed bounded listing")
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>source</Name><KeyCount>0</KeyCount><IsTruncated>false</IsTruncated></ListBucketResult>`))
	}))
	defer server.Close()
	if err := manager.ValidateMigrateConfig(MigrateConfig{SourceEndpoint: server.URL, SourceAccessKey: key.AccessKeyID, SourceSecretKey: key.SecretAccessKey, SourceBucket: "source"}); err != nil {
		t.Fatal(err)
	}
}
