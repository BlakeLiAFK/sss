# API audit remediation

Source: `SSS_API全量清单与静态审计.docx`, Library
`libfile_e0e4803abbcc8191a83c7011921d1858`, 78 operations, audited commit
`55a312a381a153bcedd8322de6e2c1c4eb18e804`. Remote HEAD was verified before work.

All 78 operations were reviewed against the report, including its 58 repair findings,
6 verification questions and health optimization. The static report is evidence
to reproduce, not proof that every endpoint has a separate defect. Shared
authentication/storage corrections resolve many inherited findings. The ledger
below distinguishes changes from retained behavior and verification limits.

## Implemented and covered by regression tests

- Real ServeHTTP/ServeMux rejects anonymous extension requests. Extensions check
  target bucket permissions, and S3 presigns retain the caller's managed key.
- `api_keys` is authoritative; Global.Auth no longer authenticates or grants
  permission. Rotation, disable, deletion and permission withdrawal affect
  subsequent signed and presigned requests, including after cache reinit.
- Signed request timestamps and payload hashes are checked. Authenticated bodies
  are streamed to bounded temporary files before dispatch, with actual sizes.
  UNSIGNED-PAYLOAD remains supported; signed chunk streaming is rejected.
- Unsupported subresources never fall through to object/bucket mutations.
  Copy checks source read access; multipart IDs bind to both bucket and key.
- Object writes/copies and multipart parts use immutable blobs. Metadata failure
  preserves the prior pointer and data. Multipart completion commits the object
  pointer and removal of upload records in one SQLite transaction.
- Logical directory pagination, literal prefixes, suffix ranges, read conditions,
  conditional creates/overwrites, multipart ordering/minimum part sizes/pagination.
- GC and integrity mutation requests reject malformed input before effects.
  Settings use one transaction after validation; password changes revoke sessions.
- Migration snapshots redact secrets and copy mutable data; cancellation interrupts
  remote I/O and preserves cancelled state. At most 16 active jobs are accepted.
- Admin errors are JSON, S3 errors remain XML; error request IDs reuse the header.
- Frontend presigning uses an authenticated admin endpoint and an enabled managed
  key with the necessary permission. If none exists, it returns 409 rather than
  minting a new credential. Preview URLs use authenticated same-origin downloads.

## Storage and operational boundaries

Existing object paths remain readable. The new `parts.storage_path` column has an
empty default; in-progress uploads from older versions use their legacy part
paths. New writes are immutable paths; deleted/replaced object blobs are reclaimed
by GC. Object publication is a SQLite pointer commit after file fsync. SQLite uses
FULL synchronous mode. Blob directories are synced before publication.

A FileStore read/write lock serializes admitted storage mutations and coordinates
migration writes against reads and maintenance. Authentication and bounded HTTP
body staging happen before that lock; login/setup/session and migration-control
handlers do not hold it while waiting on clients. Setup acquires it only for the
short runtime configuration reload. Temporary request bodies live outside the
object store, so concurrent GC cannot collect them. This intentionally favors correctness over
parallel upload throughput. Cancellation endpoints do not take that lock so an
in-flight remote transfer can be cancelled. Deploy one server process per store;
shared-directory multi-process operation is not supported.

No production storage, real credentials, releases, deployments or workflows were
changed. Tests use temporary data and generated credentials; historical test
fixtures explicitly import their test-only keys instead of relying on Global.Auth.

## Compatibility and migration

- Existing managed API keys keep their IDs and secrets. Disabled/deleted keys are
  never automatically recreated. Settings-only credentials from older deployments
  require an explicit one-time `-import-legacy-key` startup flag; remove the flag
  after successful import. It clears the legacy secret setting and does not
  overwrite an existing managed key. Do not use this flag to restore revoked keys.
- New setup commits installation, administrator and generated managed key in one
  transaction. Host, port and storage location remain command-line runtime values;
  install form fields do not relocate a live database or listener.
- Both `is_public` and legacy `isPublic` are accepted, but the flag is required and
  conflicting values are rejected. GC now requires an explicit `dry_run` boolean;
  integrity repair requires `repair: true`. The bundled UI supplies these fields.
- Object deletion is immediate in metadata; disk reclamation is deferred to GC.
  Bucket deletion rejects live objects or multipart uploads and also defers disk
  cleanup. This avoids a filesystem failure breaking the committed namespace.
- Supported key names include `file..txt`; actual traversal segments are rejected.
  New bucket names follow DNS-style naming and reserve application routes. Existing
  buckets are not renamed. Previously accepted names overlapping application
  routes remain accessible through admin APIs rather than gaining new S3 routes.
- S3 `ListBuckets` still lists bucket names for authenticated keys; write permission
  still includes bucket management. These are retained permission semantics, not
  silently changed into a new policy model.
- Multipart ordering and minimum non-final part size follow the
  [AWS CompleteMultipartUpload contract](https://docs.aws.amazon.com/AmazonS3/latest/API/API_CompleteMultipartUpload.html).
  Unsupported S3 subresources return errors rather than mutating base resources.
  AWS streaming-chunk signatures, ACLs, policies, tagging, upload-part-copy and
  listing active uploads remain unsupported.

## Operation ledger

“Retained” means no confirmed local defect from this audit; it does not certify
all deployment configurations. Evidence: `api/audit_regression_test.go`,
`auth/audit_regression_test.go`, `admin/audit_regression_test.go`,
`storage/audit_regression_test.go` plus existing package and AWS SDK tests under
`internal/`. Shared fixes apply to every operation passing through that layer.

| ID | Operation | Disposition and evidence |
|---|---|---|
| S01 | ListBuckets | Managed-key authentication; existing listing semantics retained; SDK list test. |
| S02 | CreateBucket | Managed permissions, bucket naming and serialized creation. |
| S03 | DeleteBucket | Reject nonempty/active-upload bucket; metadata deletion, deferred GC. |
| S04 | HeadBucket | Managed permissions; errors preserved. |
| S05 | ListObjects V1 | Logical delimiter pagination, literal prefixes, NextMarker and parameter validation. |
| S06 | ListObjects V2 | Same pagination fix; existing marker-style continuation token retained. |
| S07 | GetObject | Signed/public boundary, suffix ranges, conditional reads. |
| S08 | PutObject | Actual payload digest/size, immutable blob commit, serialized conditions. |
| S09 | CopyObject | Source permission and immutable destination, self-copy safe. |
| S10 | DeleteObject | Metadata-first logical deletion; subresource guard prevents accidental deletion. |
| S11 | HeadObject | Read conditions and managed/public boundary. |
| S12 | InitiateMultipartUpload | Authenticated target and strict subresource dispatch. |
| S13 | UploadPart | Upload ID bound to bucket/key; actual bytes; immutable part commit. |
| S14 | CompleteMultipartUpload | Ascending unique parts, minimum size, aggregate size, atomic metadata transaction. |
| S15 | AbortMultipartUpload | Target binding and transactional metadata removal; cleanup errors surfaced. |
| S16 | ListParts | Target binding and validated marker/max-parts pagination. |
| X01 | Presign | ServeMux auth, target permission, caller identity, AWS escaping and expiry validation. |
| X02 | Read public flag | ServeMux auth and target read permission. |
| X03 | Set public flag | ServeMux auth, write permission, required unambiguous flag. |
| X04 | Search | ServeMux auth and target read permission. |
| X05 | Object exists | ServeMux auth and target read permission. |
| H01 | Health | GET/HEAD, actual build version and metadata readiness failure. |
| U01 | Setup status | Database errors return 500 rather than uninstalled. |
| U02 | Install | Single transaction and setup lock; managed initial key; runtime fields remain CLI-controlled. |
| U03 | Reset check | Verified intentional recovery UI contract; unauthenticated marker path/existence retained. See limits. |
| U04 | Reset password | Regular-file check, all stat errors rejected, serialized one-time marker consumption. |
| A01 | Login | Retained bcrypt/session/rate-limit flow; compatibility credentials read enabled managed key only. |
| A02 | Logout | Retained current-session invalidation. |
| A03 | List API keys | Permission query errors surfaced; managed state matches authentication. |
| A04 | Create API key | Cache reload failure surfaced and fails closed. |
| A05 | API key detail | Nil/error handling; managed state authoritative. |
| A06 | Update API key | Atomic description/enabled update and fail-closed cache refresh. |
| A07 | Delete API key | Revocation effective; no Global.Auth bypass. |
| A08 | Set permissions | Permission changes effective for default and additional keys. |
| A09 | Revoke permissions | Revocation effective; cache errors surfaced. |
| A10 | Reset secret | Old secret invalidated; new managed secret accepted. |
| A11 | List buckets | Retained administrator-wide list and JSON errors. |
| A12 | Create bucket | Shared bucket validation and serialized mutation. |
| A13 | Bucket detail | Retained existence/error checks. |
| A14 | Update public alias | Shared required flag schema and audit logging. |
| A15 | Delete bucket | Nonempty/upload protection; metadata namespace commit and deferred GC. |
| A16 | Get public flag | Nil/error-safe metadata read. |
| A17 | Set public flag | Both spellings supported, missing/conflicting values rejected. |
| A18 | List objects | Retained fixed page size; shared storage pagination corrections. |
| A19 | Delete object | Logical metadata deletion; safe key semantics. |
| A20 | Upload object | Bounded multipart body, actual size, staged immutable write. |
| A21 | Download object | Escaped filename, quoted ETag, ServeContent conditions/ranges. |
| A22 | Copy object | Immutable copy supports safe self-copy and failed replacement preservation. |
| A23 | Search objects | Retained bounded administrator search semantics. |
| A24 | Batch delete | Per-object logical deletion; metadata failure is not reported as success. |
| A25 | Batch ZIP | Stage archive before response; missing/read failures abort with JSON error. |
| A26 | Preview | Same-origin authenticated download; bounded text preview retained. |
| A27 | Storage overview | Disk traversal failures surfaced. |
| A28 | Recent objects | Retained bounded limit/default behavior. |
| A29 | GC preview | Dry-run retained, age bound validated; shared lock prevents concurrent publication. |
| A30 | GC execution | Malformed/missing/overflow requests rejected before deletion; explicit dry_run required. |
| A31 | Integrity scan | Bounded limit and shared read lock; existing scan tests. Scale limits noted below. |
| A32 | Integrity repair | Explicit repair flag; stale missing-file finding rechecked before deletion. |
| A33 | Migration list | Copied snapshots, source secret redacted. |
| A34 | Create migration | 16 active-job limit; local signed SDK and cancellation tests, isolated byte-transfer test. |
| A35 | Validate migration | Local signed ListObjectsV2(max-keys=1) test; existing 10-second timeout retained. |
| A36 | Migration detail | Copied snapshot and redaction, including mutable slices/timestamps. |
| A37 | Delete migration record | Retained rejection for pending/running jobs. |
| A38 | Cancel migration | Per-job cancellation interrupts I/O; atomic cancellation/publication boundary. |
| A39 | Audit log query | Strict timestamp/boolean/page/limit validation. |
| A40 | Audit stats | Retained read-only aggregation and error handling. |
| A41 | Read settings | Retained runtime read-only and secret-free response. |
| A42 | Update settings | Full validation before transactional settings update. |
| A43 | Change password | Whitespace preserved and all sessions revoked. |
| A44 | GeoIP status | Retained read-only status. |
| A45 | Upload GeoIP | Unique staging file; open candidate and atomically replace before swapping reader. |
| A46 | Delete GeoIP | File deletion succeeds before reader closes; request statistics already gate on GeoIP availability. |
| A47 | Check update | HTTPS GitHub source and timeout inspected; mocked 200/404/403 transport tests. No cache added. |
| A48 | Read GeoStats config | Retained explicit configured enabled state and GeoIP availability fields. |
| A49 | Update GeoStats config | Validate then transaction; running ticker restarts on interval change. |
| A50 | GeoStats data | Strict group/date range validation; existing bounded queries retained. |
| A51 | Delete GeoStats data | Strict boolean/date checks and conflicting modes rejected; omitted mode retains retention cleanup. |
| A52 | GeoStats summary | Strict date and start/end validation. |

## Validation (2026-10-10, macOS arm64)

- `go test ./... -count=1 -coverprofile=/tmp/sss-coverage.out`: passed. Total
  statement coverage 65.8% (includes command/tool packages at 0%). Package coverage:
  admin 64.2%, api 73.1%, auth 71.9%, config 80.7%, storage 68.9%, utils 59.8%.
  Coverage is statement execution, not proof of all failure modes.
- `go test -race ./... -count=1`: passed all packages.
- `go vet ./...`: passed.
- `go test ./internal/api -run TestAWSSDK -count=1`: passed, real AWS Go SDK with
  isolated local HTTP servers. No production services or data.
- `cd web && npm ci && npm run build`: strict vue-tsc and Vite build passed.
  Minimal Element Plus tag/switch typing fixes restore the strict build.
- `go build -tags embed -o /tmp/sss-audit-server ./cmd/server`: passed using the built frontend.
- Existing flaky signature-tampering test now always changes the signature;
  migration cancellation fixtures use owned local servers rather than localhost:9000.

## Remaining verification boundaries

No confirmed audit defect is intentionally left unfixed. These are explicit
limits of the performed verification, not a claim of complete AWS S3 support:

- No power-cut/crash-injection campaign, huge-store load test, browser interaction
  E2E, cross-platform filesystem runtime or multi-process store validation. FULL
  SQLite synchronization and file/directory fsync improve durability but do not
  prove behavior under every filesystem/device failure. The mutation lock reduces
  concurrent write throughput. GC still scans the store and may consume substantial
  memory on large datasets.
- U03 retains the unauthenticated reset-marker absolute path/existence response
  needed by the recovery UI. A locally created marker authorizes recovery; treat
  this as an operator-controlled recovery window. If password persistence fails
  after consuming the marker, recreate the marker to retry. No new production
  reverse-proxy policy was applied.
- A47 uses the fixed HTTPS GitHub API, a finite timeout and no cache; upstream
  availability/rate limits are surfaced as errors. Tests mock upstream responses,
  not live GitHub availability. It checks metadata only, never installs an update.
- GeoStats configured enabled state is preserved when GeoIP is removed; actual
  recording stops because requests require both services available. Uploading a
  valid database restores availability without discarding configured preferences.
- `npm ci` reports 16 existing dependency vulnerabilities (4 moderate, 12 high),
  and Vite reports a large output chunk. Dependency upgrades/bundle redesign were
  not included in this API remediation; neither prevented the strict build.
- No release, deployment, tag or workflow change. The existing release workflow
  runs only on `v*` tags or manual dispatch, not a master branch push.

## Independent review follow-up

The independent review after `f9764a4` found additional boundaries; the earlier
completion statement was too broad. This follow-up adds these corrections and
regressions:

1. **Unsigned operation-header injection:** reproduced a host-only PUT presigned
   URL accepting an appended `x-amz-copy-source` and returning CopyObject success
   using the signing key's private-source access. Both query and header SigV4 now
   require every present `x-amz-*` header to be in the signed set. The S3 payload
   hash exception remains bound by the header-auth canonical payload line;
   presigned requests only permit an unsigned `UNSIGNED-PAYLOAD` marker. Signed
   header names must be lower-case, unique, sorted and include host. Tests reject
   injected copy, ACL, tagging and metadata headers. A real AWS Go v4 signer
   presigned copy succeeds with its original headers and rejects a tampered source.
   The full AWS SDK suite still passes. This follows the
   [AWS canonical-header requirements](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_sigv-create-signed-request.html).
   Ordinary optional HTTP Content-Type/Range/conditional headers keep existing
   behavior; adding them cannot turn an upload into a source-reading operation.
2. **Slow-client lock denial of service:** request bodies are admitted and staged
   with byte limits before the storage consistency lock. Authenticated writes
   recheck authentication and bucket permissions when dispatched after staging.
   The deterministic `TestAuditSlowRequestDoesNotHoldStorageLock` blocks login,
   setup and upload readers with channels and verifies health and object reads
   complete before releasing the blocked body. Existing concurrent conditional
   create and storage/GC tests continue to cover the retained consistency boundary.
   Short configuration snapshots still use the lock; this is not a guarantee that
   health checks ignore all legitimate storage activity or filesystem stalls.
3. **Copy source conditions:** CopyObject enforces source ETag and modified /
   unmodified timestamps before staging or replacing its target, returning 412
   when a condition fails. A matching source If-Match takes precedence over a
   failing If-Unmodified-Since; a failing If-None-Match is not overridden by a
   passing If-Modified-Since. Malformed dates return 400. The source and destination
   checks execute under the same mutation lock. Tests cover both mixed cases,
   individual failures/successes and unchanged destination bytes, following
   [the AWS CopyObject conditions](https://docs.aws.amazon.com/AmazonS3/latest/API/API_CopyObject.html).
4. **Multipart rollback evidence:** `TestAuditMultipartCommitRollbackAndRetry`
   injects an SQLite trigger failure at deletion of parts, after the transaction
   has inserted the replacement object pointer. It verifies the old pointer and
   bytes, upload record, part record and part bytes all survive; removing the
   trigger allows a merge/commit retry to publish the intended replacement.

Final follow-up validation uses `/tmp/sss-review-coverage.out` and includes
uncached `go test ./...`, the AWS SDK tests, `go test -race ./...`, `go vet ./...`
and the embedded build. No frontend changes were made in this follow-up; the
previous strict frontend build remains applicable.
