package storage

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The helper deliberately never closes SQLite or the blob store. The parent
// kills it at an acknowledged persistence boundary, then opens the same files.
func TestProcessCrashRecovery(t *testing.T) {
	if root := os.Getenv("SSS_CRASH_TEST_ROOT"); root != "" {
		fs, ms := openCrashStore(t, root)
		path, etag, err := fs.PutObject("crash", "key", strings.NewReader("replacement"), 11)
		if err != nil {
			t.Fatal(err)
		}
		if os.Getenv("SSS_CRASH_TEST_PHASE") == "published" {
			if err := ms.PutObject(&Object{Bucket: "crash", Key: "key", StoragePath: path, Size: 11, ETag: etag, LastModified: time.Now()}); err != nil {
				t.Fatal(err)
			}
		}
		fmt.Println("CRASH_BOUNDARY_READY")
		for {
			time.Sleep(time.Hour)
		} // Parent kills without Go defers or DB Close.
	}
	for _, phase := range []string{"staged", "published"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			fs, ms := openCrashStore(t, root)
			if err := ms.CreateBucket("crash"); err != nil {
				t.Fatal(err)
			}
			old, etag, err := fs.PutObject("crash", "key", strings.NewReader("original"), 8)
			if err != nil {
				t.Fatal(err)
			}
			if err := ms.PutObject(&Object{Bucket: "crash", Key: "key", StoragePath: old, Size: 8, ETag: etag, LastModified: time.Now()}); err != nil {
				t.Fatal(err)
			}
			if err := ms.Close(); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestProcessCrashRecovery$")
			cmd.Env = append(os.Environ(), "SSS_CRASH_TEST_ROOT="+root, "SSS_CRASH_TEST_PHASE="+phase)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill() }()
			ready := make(chan bool, 1)
			go func() {
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					if scanner.Text() == "CRASH_BOUNDARY_READY" {
						ready <- true
						return
					}
				}
				ready <- false
			}()
			select {
			case ok := <-ready:
				if !ok {
					t.Fatal("child exited before persistence boundary")
				}
			case <-time.After(20 * time.Second):
				t.Fatal("child boundary timed out")
			}
			if err := cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Wait(); err == nil {
				t.Fatal("child was not terminated")
			}
			fs, ms = openCrashStore(t, root)
			defer ms.Close()
			obj, err := ms.GetObject("crash", "key")
			if err != nil || obj == nil {
				t.Fatalf("reopen pointer: %v", err)
			}
			want := "original"
			if phase == "published" {
				want = "replacement"
			}
			data, err := os.ReadFile(obj.StoragePath)
			if err != nil || string(data) != want {
				t.Fatalf("reopen contents %q: %v", data, err)
			}
			if (obj.StoragePath == old) != (phase == "staged") {
				t.Fatal("wrong committed pointer after kill")
			}
			// Each boundary leaves exactly one unreachable immutable blob. GC must
			// reclaim that blob while retaining whichever pointer committed.
			result, err := RunGC(fs, ms, 24*time.Hour, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.OrphanCount != 1 {
				t.Fatalf("orphans=%d, want 1", result.OrphanCount)
			}
			data, err = os.ReadFile(obj.StoragePath)
			if err != nil || string(data) != want {
				t.Fatalf("GC lost live blob: %v", err)
			}
			again, err := RunGC(fs, ms, 24*time.Hour, true)
			if err != nil || again.OrphanCount != 0 {
				t.Fatalf("GC not complete: %+v %v", again, err)
			}
		})
	}
}

func openCrashStore(t *testing.T, root string) (*FileStore, *MetadataStore) {
	t.Helper()
	fs, err := NewFileStore(filepath.Join(root, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	ms, err := NewMetadataStore(filepath.Join(root, "metadata.db"))
	if err != nil {
		t.Fatal(err)
	}
	return fs, ms
}

func TestFilesystemFailurePreservesPublishedObject(t *testing.T) {
	root := t.TempDir()
	fs, ms := openCrashStore(t, root)
	defer ms.Close()
	if err := ms.CreateBucket("crash"); err != nil {
		t.Fatal(err)
	}
	old, etag, err := fs.PutObject("crash", "key", strings.NewReader("original"), 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := ms.PutObject(&Object{Bucket: "crash", Key: "key", StoragePath: old, Size: 8, ETag: etag, LastModified: time.Now()}); err != nil {
		t.Fatal(err)
	}
	// A file where a bucket directory is required exercises a real filesystem
	// failure on all platforms, including users for whom chmod cannot deny writes.
	if err := os.WriteFile(filepath.Join(fs.basePath, "blocked"), []byte("obstruction"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fs.PutObject("blocked", "key", strings.NewReader("replacement"), 11); err == nil {
		t.Fatal("expected filesystem failure")
	}
	ms.db.SetMaxOpenConns(1)
	if _, err := ms.db.Exec("PRAGMA query_only = ON"); err != nil {
		t.Fatal(err)
	}
	staged, etag, err := fs.PutObject("crash", "key", strings.NewReader("replacement"), 11)
	if err != nil {
		t.Fatal(err)
	}
	if err := ms.PutObject(&Object{Bucket: "crash", Key: "key", StoragePath: staged, Size: 11, ETag: etag, LastModified: time.Now()}); err == nil {
		t.Fatal("expected SQLite read-only commit rejection")
	}
	if err := fs.DeleteObject(staged); err != nil {
		t.Fatal(err)
	}
	obj, err := ms.GetObject("crash", "key")
	if err != nil || obj.StoragePath != old {
		t.Fatal("failed write changed pointer", err)
	}
	data, err := os.ReadFile(old)
	if err != nil || string(data) != "original" {
		t.Fatal("failed write damaged original", err)
	}
}
