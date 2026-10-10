//go:build windows

package storage

import (
	"fmt"
	"os"
)

// Windows does not provide the Unix directory-fsync contract on os.Open's
// read-only directory handle (FlushFileBuffers requires GENERIC_WRITE). Do not
// turn every successful blob write into an access-denied failure. Each blob's
// writable handle is still Sync'ed before SQLite FULL publication. This is not
// a claim of power-loss durability for directory entries on Windows; validate
// the target volume with scripts/verify-filesystem.ps1 before deploying there.
func syncDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	return nil
}
