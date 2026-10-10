# Local administrator recovery

Recovery still requires the server operator to authorize it on the server's
filesystem. No API creates this authorization, and no real deployment or
credential is needed by the tests.

1. In the server process's working directory, create an empty
   `data/.reset_password` regular file. On POSIX systems:
   `touch data/.reset_password`. On Windows PowerShell:
   `New-Item -ItemType File data/.reset_password`.
2. Open **Forgot password** on the login screen. The dialog checks authorization
   immediately; use **Check file** after creating it. Enter a password of at least
   eight characters containing an uppercase letter, lowercase letter and digit.
3. After successful recovery, all active admin sessions are invalidated. Sign in
   with the new password. The authorization file is normally removed.

The unauthenticated check returns only `file_exists`; it no longer returns
`file_path` or `command`. These removed response fields intentionally prevent
exposure of deployment-specific absolute paths. Older frontends should be
updated with the server. The current frontend supplies fixed local instructions
and distinguishes expired authorization (403, return to instructions) from a
storage failure (500, preserve the form for retry).

Existing empty authorization files are compatible. Before changing a password,
the server writes and flushes a random nonce to that existing file. The database
then commits the password hash and nonce consumption together. A failed database
transaction preserves both the old password and unused authorization, so the
same request can be retried. A consumed nonce is rejected after restart even if
a crash or permission error left its file behind. Such a file must be removed
by the operator before creating a new empty authorization file; merely touching
an existing consumed file does not reauthorize it. Malformed, partially written,
non-regular, or symlink markers are rejected; the operator must replace these
with a new empty regular file. Do not edit/truncate a marker during a reset.

Regression tests:

- `go test ./internal/admin ./internal/storage -run 'TestRecovery|TestHandleResetPassword' -count=1`
- `TestRecoveryTransactionFailureAndReplay` injects a SQLite trigger failure
  after the consumption insert, verifies rollback preserves the original
  password and unused token, retries successfully, then reopens the database
  and verifies replay rejection.
- `TestRecoveryMarkerRetryReplayAndPrivacy` checks anonymous output, legacy empty
  markers, a retained consumed file, and authorization retention on database
  failure. `TestRecoveryRejectsNonRegularMarkers` covers directories and symlinks.

These tests verify logical failure and restart behavior. They do not simulate a
physical power cut or prove filesystem/hardware flush guarantees.
