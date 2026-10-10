//go:build darwin || linux

package storage

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// RLIMIT_FSIZE causes the kernel write syscall to fail with EFBIG. The limit
// exists only in this isolated subprocess; it does not fill any host volume.
func TestKernelWriteFailureCleansUnpublishedBlob(t *testing.T) {
	if root := os.Getenv("SSS_WRITE_FAILURE_ROOT"); root != "" {
		fs, err := NewFileStore(root)
		if err != nil {
			t.Fatal(err)
		}
		old, _, err := fs.PutObject("crash", "key", strings.NewReader("original"), 8)
		if err != nil {
			t.Fatal(err)
		}
		signal.Ignore(syscall.SIGXFSZ)
		var previous syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &previous); err != nil {
			t.Fatal(err)
		}
		limited := previous
		limited.Cur = 0
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limited); err != nil {
			t.Fatal(err)
		}
		_, _, writeErr := fs.PutObject("crash", "key", strings.NewReader("replacement"), 11)
		restoreErr := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &previous)
		if restoreErr != nil {
			t.Fatal(restoreErr)
		}
		if !errors.Is(writeErr, syscall.EFBIG) {
			t.Fatalf("expected kernel EFBIG, got %v", writeErr)
		}
		data, err := os.ReadFile(old)
		if err != nil || string(data) != "original" {
			t.Fatal("old blob damaged", err)
		}
		files, err := filepath.Glob(filepath.Join(filepath.Dir(old), ".blob-*"))
		if err != nil || len(files) != 1 {
			t.Fatalf("unpublished failed blob remains: %v %v", files, err)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestKernelWriteFailureCleansUnpublishedBlob$")
	cmd.Env = append(os.Environ(), "SSS_WRITE_FAILURE_ROOT="+t.TempDir())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("isolated kernel failure test: %v\n%s", err, output)
	}
}
