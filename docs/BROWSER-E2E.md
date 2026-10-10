# Browser end-to-end verification

Run from the repository with Node.js and Go installed:

```sh
cd web
npm ci
npx playwright install chromium
E2E_ARTIFACT_DIR=/tmp/sss-browser-evidence npm run test:e2e
```

`tools/browser-e2e.mjs` performs the strict production frontend build, builds the Go server, chooses an available loopback port, and starts it in a newly created temporary directory. The database, object store, administrator credentials, local recovery authorization, and uploaded bytes are generated solely for the test. It does not connect to a configured or production SSS instance.

The script drives a real headless Chromium browser through first-run installation, login, private bucket creation, upload, private text preview, exact-byte download, settings persistence, logout, protected-route rejection, local-operator password recovery, and login with the changed password. The recovery UI retry test deliberately intercepts the first reset response with HTTP 500; the subsequent retry reaches the real local server. Storage transaction failure and restart semantics are covered separately by Go tests.

A JSON result records the platform, actual browser version, passed steps, and uncaught browser errors. A Playwright trace is written on success and a screenshot on failure. Traces may contain generated temporary credentials; retain them locally, not in a public repository. The script terminates the server and browser. With `E2E_ARTIFACT_DIR` set, it removes temporary server state unless `E2E_KEEP_TEMP=1` is explicitly supplied for debugging. Without an artifact directory, evidence and the temporary state remain together in the printed temporary location.

`E2E_SKIP_BUILD=1` can reuse already built frontend assets for iteration; final verification should omit it. `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` can explicitly select a locally installed Chromium, but the default downloaded browser matches the pinned Playwright version. Passing on macOS does not assert that Windows/Linux browsers or filesystems were exercised.

## Executed result (2026-10-10)

On macOS arm64, the strict build and all 11 browser steps passed using actual Chromium **156.0.8078.4**, with zero uncaught browser runtime errors. `docs/evidence/browser-e2e-macos.json` records the machine-readable result. Dashboard ECharts canvas rendering and both Chinese/English locale transitions were exercised after the dependency upgrades. The private-object download assertion compares the bytes saved by the browser with the generated uploaded bytes.

The run exposed and fixed two user-visible defects: the upload confirmation used a nonexistent translation key, and the Download button opened text objects inline. Downloads now use the existing same-origin administrator attachment endpoint with the session cookie, streaming to the browser without accumulating the complete object in JavaScript memory.
