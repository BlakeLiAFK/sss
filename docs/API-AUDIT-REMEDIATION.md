# API audit remediation

Source: `SSS_API全量清单与静态审计.docx`, Library
`libfile_e0e4803abbcc8191a83c7011921d1858`, 78 operations, audited commit
`55a312a381a153bcedd8322de6e2c1c4eb18e804`. Remote HEAD was verified before work.

This is an ongoing remediation record. The initial patch passes `go test ./...`;
race, vet, frontend, further contract checks and the final operation ledger are
pending. The static report is evidence to reproduce, not proof of a defect in
every endpoint.

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

A FileStore read/write lock serializes HTTP mutations and coordinates migration
writes against reads and maintenance. This intentionally favors correctness over
parallel upload throughput. Cancellation endpoints do not take that lock so an
in-flight remote transfer can be cancelled. Deploy one server process per store;
shared-directory multi-process operation is not supported.

No production storage, real credentials, releases, deployments or workflows were
changed. Tests use temporary data and generated credentials; historical test
fixtures explicitly import their test-only keys instead of relying on Global.Auth.
