#!/usr/bin/env sh
# Run natively on Linux/macOS. This is execution, not cross-compilation.
set -eu
cd "$(dirname "$0")/.."
actual=$(go env GOHOSTOS)
target=$(go env GOOS)
if [ "$actual" != "$target" ]; then
  echo "Refusing cross-target run: host=$actual target=$target" >&2
  exit 1
fi
printf 'Native filesystem verification: '
go version
uname -a
# Tests use t.TempDir. Set TMPDIR to the filesystem being qualified if needed.
printf 'Temporary-files filesystem: '
case "$actual" in
  darwin) df -h "${TMPDIR:-/tmp}" ;;
  *) df -T "${TMPDIR:-/tmp}" ;;
esac
go test ./internal/storage -count=1 -timeout=5m -run 'Test(ProcessCrashRecovery|FilesystemFailurePreservesPublishedObject|KernelWriteFailureCleansUnpublishedBlob|AuditImmutableBlobFailureAndSelfCopy|AuditMultipartCommitRollbackAndRetry|AuditMultipartColumnMigration)'
go test ./internal/storage -count=1 -timeout=10m
