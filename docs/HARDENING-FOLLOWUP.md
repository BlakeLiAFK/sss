# Remaining-item hardening and validation

Date: 2026-10-10. Baseline: `be66c1a53b6a51b71173ad67c11b386c3b3d1595`.
The checkout was clean and remote master matched before work. This follow-up
addresses the four additional workstreams requested after the API audit.
All data, credentials, subprocesses and containers used for validation were
isolated and disposable. No production configuration, deployment, release, tag
or GitHub workflow was modified or manually triggered.

## Dependency and browser work

Full npm audit: **16 → 0 vulnerabilities**. Production-only audit: **11 → 0**.
The original full count was 12 high / 4 moderate; production was 9 high / 2 moderate.
Supported dependency upgrades include Vite 8, Vue I18n 11, ECharts 6 and current
compatible application/transitive patches. Node requirements are explicit; `make build` now uses lockfile-based `npm ci`
and the strict typechecked build. This
is not a suppressed audit, forced incompatible resolution or disabled type check.
See [dependency report](DEPENDENCY-REMEDIATION.md) and full before/after JSON in
[evidence](evidence/). `npm ci`, strict `npm run build`, and both final audit
commands passed. The remaining Vite large-chunk warning is visible and is a
bundle-size observation, not an unresolved npm advisory.

Real Chromium 156.0.8078.4 on macOS arm64 passed 11 browser workflows with zero
runtime errors: installation, login, private bucket creation, upload/list,
authenticated preview, exact-byte download, charts/locales, settings persistence,
logout, local recovery authorization and retry. E2E found and fixed a missing
upload-button translation and a Download button that previously opened text
inline. Downloads now use the existing same-origin authenticated attachment
stream. See [browser report](BROWSER-E2E.md), the reproducible
[runner](../tools/browser-e2e.mjs) and
[redacted results](evidence/browser-e2e-macos.json). Sensitive generated-credential
browser traces stay outside the repository.

## Recovery authorization

The anonymous reset check returns availability only, with no absolute path or
shell command. The operator still creates the local marker; the application does
not grant recovery authorization from the network. Existing empty markers are
upgraded to a durable random nonce. Password update and nonce consumption commit
in one SQLite transaction. A failed transaction can be retried; a crash or failed
marker removal after success cannot replay authorization. The UI keeps form input
on recoverable failure and rechecks authorization after 403. Tests inject a
transaction failure, reopen the database and check replay rejection. See
[operator migration and retry instructions](PASSWORD-RECOVERY.md).

## Storage concurrency

The whole-store gate remains a reader/writer maintenance boundary. Normal S3
object mutations and single-object admin upload/delete take its shared side, plus
an exclusive logical `(bucket,key)` lock. Distinct object writes can progress
concurrently; the same key's condition checks and publication remain serialized.
Key-lock entries are reference-counted and removed when no operation uses them.
Migration publication uses the same key locks. Bucket mutations, compound admin
operations, settings, GC preview/deletion and repair retain the exclusive gate.
Immutable source snapshots keep CopyObject stable while other keys change.

HTTP bodies are authenticated and staged outside this boundary, as in the prior
review fix. GC cannot collect an unpublished blob or an in-use read because the
operation retains a shared maintenance lease. Same-key conditional-create,
independent-key progress, copy-source conditions, migration cancellation and
multipart rollback/retry regressions all remain in the suite. Multiple server
processes sharing a store are still unsupported.

### Concurrent request benchmark

Same handler-level benchmark, generated managed credentials and temporary store;
three repetitions on Apple M4 Pro, Go 1.25.3. Seven of eight operations are writes
to distinct keys, the eighth reads a pre-existing small object.

| Payload | Setup | Median before ms/op | Median after ms/op | Observation |
|---|---|---:|---:|---|
| 64 KiB | 24 operations, 8 CPUs | 5.307 | 5.297 | Essentially unchanged |
| 4 MiB | 24 operations, 8 CPUs | 12.443 | 9.351 | About 25% less elapsed time/op |
| 64 MiB | 8 operations, 4 CPUs | 122.246 | 97.749 | About 20% less elapsed time/op |

[Small/medium before](evidence/performance-before.txt),
[after](evidence/performance-after.txt),
[large-object method](evidence/performance-large-method.md) and raw large-object
logs include all results. These are short, in-process measurements, not a network,
TLS or production capacity claim. Raw benchmark MB/s counts a full payload even
for the small GET, so it must not be reported as actual network throughput.

## GC memory

GC no longer materializes all live object metadata or all active upload IDs.
It walks directories in 256-entry batches and queries an indexed storage-path
membership set for one batch at a time. Expired uploads use keyset pagination.
Totals cover the complete scan, while response detail arrays sample at most 1,000
entries and explicitly set `details_truncated`; the frontend explains the sample.
Deletion streams independently of those arrays, so items beyond the sample are
still reclaimed. Empty directories are pruned after closing their handles. The
store root and directories containing live files remain intact. An automatic
`objects(storage_path)` index migration incurs one-time startup work on old stores.

The regression fixture includes more than 1,000 orphans and expired uploads,
checks complete cleanup and sample bounds, and preserves live objects/uploads.
Expired-upload metadata deletion is transactional and filesystem failures are
reported instead of ignored.

| Files | Natural sampled heap peak increment, before → after | Approx. post-GC live increment, before → after |
|---:|---:|---:|
| 20,000 | 13.79 → 2.90 MiB | 5.86 → 0.36 MiB |
| 80,000 | 51.16 → 3.05 MiB | 27.36 → 0.45 MiB |

The [memory probe and raw records](evidence/gc-memory/README.md) explain the
sampling, forced-GC perturbation and exact source hashes. These are sampled Go
heap deltas, not strict upper bounds or RSS. Work-dependent cumulative allocation
remains proportional to scan size. The tradeoff is real: the 20k-object benchmark
median increased from 83.67 to 110.01 ms (about 31% slower); the separate memory
probe observed 36–38% slower scans. The improvement is bounded retained memory,
not faster GC. Directory depth, SQLite native/cache memory and large-store wall
time remain relevant operational considerations.

## Failure and platform validation

New child-process tests force termination at two publication boundaries and reopen
SQLite without a graceful shutdown. Additional tests exercise a filesystem
obstruction, SQLite read-only commit failure, transaction-trigger rollback and
actual Unix kernel `EFBIG` writes via a child-only file-size limit. Previously
published bytes survive failed writes; unpublished blobs are safely collectable.
See [full filesystem evidence](evidence/filesystem-validation.md).

| Environment actually run | Coverage |
|---|---|
| macOS arm64 / APFS | Full Go suite, race detector, vet, process/filesystem faults, frontend build, Chromium E2E, performance experiments |
| Linux arm64 / Docker overlay | Full Go suite, filesystem/process faults, final storage suite including empty-directory GC follow-up |
| Native Windows / NTFS | Not run: no Windows host or VM is available. Executable PowerShell runner supplied; no passing claim |

The Linux run used a pinned existing Go image, source streamed into a disposable
container, and no writable host mounts. [Linux log](evidence/linux-final.log)
includes the full-suite output and final storage rerun. Scripts
[`verify-filesystem.sh`](../scripts/verify-filesystem.sh) and
[`verify-filesystem.ps1`](../scripts/verify-filesystem.ps1) reject cross-target
execution. Cross-compilation does not count as a platform runtime pass.

The Windows storage test executable also cross-compiles successfully, but was
not executed. Platform-specific directory synchronization is now explicit:
Unix retains directory fsync; Windows validates the directory but relies on
writable blob-handle Sync plus SQLite FULL, without claiming a Unix directory
fsync barrier. The prior read-only-directory Sync was incompatible with
[FlushFileBuffers' write-access requirement](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers).
Windows directory-entry power-loss durability is therefore a documented capability
limit requiring native volume qualification, not a passed test or ignored Unix
I/O error.

Physical power-loss/hardware-cache behavior, a genuinely full volume (`ENOSPC`),
native Windows NTFS, Linux native ext4/XFS, Linux/Windows browsers and sustained
production load were not exercised. `Process.Kill` is process death, `EFBIG` is a
real kernel write failure, and Docker overlay is a real Linux filesystem run;
none is relabeled as another kind of evidence. No additional infrastructure or
production permission was assumed to close those gaps.

## Final command record

- `go test ./... -count=1 -coverprofile=/tmp/sss-fixall-coverage.out`: passed.
- `go test -race ./... -count=1 -coverprofile=/tmp/sss-fixall-final-coverage.out`: passed.
- `go vet ./...`: passed.
- `go build -tags embed -o /tmp/sss-fixall-server ./cmd/server`: passed with the final built frontend.
- `npm ci`, `npm run build`, full and production npm audit: passed.
- `E2E_ARTIFACT_DIR=/tmp/sss-e2e-evidence node tools/browser-e2e.mjs`: passed.
- Linux `go test ./... -count=1 -timeout=10m`, then final `go test ./internal/storage -count=1 -timeout=10m`: passed.

Final statement coverage is **66.2%**, including command/tool packages at 0%.
Package coverage: admin 64.9%, API 73.2%, auth 71.9%, config 80.7%,
storage 69.2%, utils 59.8%. The [final Go log](evidence/go-final.log) records
the uncached full-suite race+coverage pass. Statement coverage is not a promise that every device failure
or S3 extension is covered.
