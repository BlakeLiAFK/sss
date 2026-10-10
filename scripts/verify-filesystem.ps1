# Run from PowerShell on a real Windows host with Go >= 1.24 installed.
# Set TEMP/TMP to a directory on the volume being qualified (for example NTFS).
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
if ((go env GOHOSTOS) -ne 'windows' -or (go env GOOS) -ne 'windows') {
    throw 'This script requires native Windows execution; cross-compilation is not verification.'
}
go version
if ($LASTEXITCODE -ne 0) { throw 'go version failed' }
[System.Environment]::OSVersion.VersionString
"TEMP=$env:TEMP"
Get-Volume | Select-Object DriveLetter, FileSystem, HealthStatus
# The kernel write-limit test is Unix-specific; the Windows suite instead
# exercises directory obstruction, read-only SQLite failure, process kill,
# reopen, immutable replacement, multipart rollback and GC pointer safety.
go test ./internal/storage -count=1 -timeout=5m -run 'Test(ProcessCrashRecovery|FilesystemFailurePreservesPublishedObject|AuditImmutableBlobFailureAndSelfCopy|AuditMultipartCommitRollbackAndRetry|AuditMultipartColumnMigration)'
if ($LASTEXITCODE -ne 0) { throw 'Filesystem regression tests failed' }
go test ./internal/storage -count=1 -timeout=10m
if ($LASTEXITCODE -ne 0) { throw 'Storage suite failed' }
