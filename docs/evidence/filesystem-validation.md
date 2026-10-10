# Filesystem failure and process-crash validation

Executed 2026-10-10 with temporary generated data only. No production storage,
credentials, deployment, release, or workflow was used.

## New executable coverage

- `TestProcessCrashRecovery/staged`: a child fsyncs a replacement immutable blob,
  acknowledges the boundary, and is killed by its parent before publishing its
  pointer. A newly opened SQLite connection still resolves the old bytes. GC
  removes the unpublished blob and preserves the old committed blob.
- `TestProcessCrashRecovery/published`: the same kill occurs after metadata
  publication, without running database Close or Go defers. Reopening resolves
  the new bytes; GC removes the superseded blob and preserves the new one.
- `TestFilesystemFailurePreservesPublishedObject`: a filesystem directory
  obstruction and a SQLite read-only rejection leave the original object intact;
  only the staged replacement is removed on rollback.
- `TestKernelWriteFailureCleansUnpublishedBlob`: on Linux/macOS, a subprocess
  applies `RLIMIT_FSIZE=0`, proving the actual kernel write returns `EFBIG`.
  The failed temporary blob is removed and the previous blob remains intact.
  This does not fill the host disk or change the parent process's limits.
- Existing multipart transaction-trigger rollback/retry and legacy schema
  migration regressions are included in both platform scripts.

## Reproducible commands

Run `scripts/verify-filesystem.sh` on native Linux/macOS, or
`powershell -File scripts/verify-filesystem.ps1` on native Windows with Go 1.24+.
The scripts reject a cross-target GOOS configuration, report the execution
platform, run the fault subset, and then run the complete storage test package.
Set `TMPDIR` (Unix) or `TEMP`/`TMP` (Windows) to the volume to be qualified.

Linux isolation used an existing local image and a disposable container:

```sh
COPYFILE_DISABLE=1 tar -cf - go.mod go.sum cmd tools internal scripts | docker run --rm -i \
  golang:1.25.12-bookworm sh -c \
  'mkdir -p /tmp/sss && cd /tmp/sss && tar -xf - && scripts/verify-filesystem.sh && go test ./... -count=1 -timeout=10m'
```

No host source/dependency directories are writable by this container. Public Go
modules are downloaded inside its disposable filesystem.

## Actual execution evidence

| Environment | Filesystem | Result |
| --- | --- | --- |
| Go 1.25.3, Darwin 25.5.0, arm64 | APFS (`/dev/disk3s5`, local, journaled) | Fault subset passed (0.674s), complete storage package passed (4.795s) |
| Go 1.25.12, Linux arm64, Docker Desktop | Linux overlay filesystem | Final fault subset passed (0.217s), complete storage package passed (8.800s); all Go packages passed |
| Windows | Not available on this host | Native execution not performed; PowerShell runner supplied |

Linux image digest:
`golang@sha256:6359592445455f2dbe2412bed411336035bc019a50017720d77454ffdd6d0f82`.
The final Linux run includes the precise `EFBIG` assertion, object-lock changes,
and streaming GC regressions. The full `go test ./... -count=1` suite passed;
see [linux-final.log](linux-final.log) for platform and package results.
These timings are test durations, not performance benchmark results.

## Limits of the evidence

Process termination tests cover OS process death and SQLite reopen, not physical
power loss, storage-controller caches, hardware failure, or torn sectors. The
kernel error test exercises `EFBIG`, not a filled disk's `ENOSPC`, and read-only
SQLite is not a simulated hardware I/O failure. Linux overlay execution does not
qualify a native ext4/XFS volume or a network filesystem. Windows NTFS semantics
remain unexecuted; a cross-compiled binary would not close that gap. No
Docker daemon settings or pre-existing containers were modified.

Final storage-only revalidation after adding GC empty-directory pruning and its
regression assertion passed on the same Linux image (`go test ./internal/storage
-count=1 -timeout=10m`). The final duration is recorded in `linux-final.log`;
other packages retain the preceding complete-suite evidence.

## Windows compile-only follow-up

`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/storage -o /tmp/sss-storage-windows.test.exe`
passes compilation. The executable was **not run**. Directory synchronization now
has platform-specific implementations: Unix keeps the original fsync; Windows
validates the directory and retains writable-file Sync plus SQLite FULL, without
calling FlushFileBuffers on an incompatible read-only directory handle. This
avoids an inappropriate platform call; it does not establish Windows directory
entry persistence under power loss. Native NTFS runtime validation is still open.

After splitting directory-sync implementations by platform, the unchanged Unix
implementation passed native Linux targeted process-crash, kernel `EFBIG`, and
streaming-GC regressions again (see `linux-final.log`). Windows remains unexecuted;
its implementation syncs writable blob handles and uses SQLite FULL but does not
claim Unix directory-fsync semantics or directory-entry power-loss durability.
