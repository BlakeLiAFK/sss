# Frontend dependency remediation — 2026-10-10

The full npm advisory response is preserved, before and after remediation, under `docs/evidence/npm-audit-*.json`. These are registry results at execution time, not a guarantee about advisories published later. No advisories were suppressed, no `npm audit --force` was used, and no workflow was changed.

| Audit scope | Before | After |
| --- | --- | --- |
| Full dependency tree | 16 (12 high, 4 moderate) | 0 |
| Production dependency tree (`--omit=dev`) | 11 (9 high, 2 moderate) | 0 |

Production-tree findings included Axios and its form-data/follow-redirects dependencies, Vue/server-renderer, ECharts, lodash/lodash-es, nanoid, postcss and source-map-js. A production-tree dependency is not proof its vulnerable code path runs in the shipped browser bundle: server-renderer, for example, is not used for server-side rendering here. The other five affected packages were development/build-only: plugin-vue, Vite, esbuild, Rollup and picomatch. All were remediated regardless of exploitability in this application.

## Changes and compatibility

| Direct package | Previous installed | New installed |
| --- | --- | --- |
| Axios | 1.13.2 | 1.20.0 |
| Vue | 3.5.25 | 3.5.43 |
| ECharts | 5.6.0 | 6.1.0 |
| Element Plus | 2.11.9 | 2.14.7 |
| Vue I18n | 9.14.5 | 11.4.13 |
| Vite | 5.4.21 | 8.3.4 |
| @vitejs/plugin-vue | 4.6.2 | 6.0.9 |
| vue-tsc | 3.1.8 | 3.3.12 |
| Playwright (new, development only) | — | 1.64.0 |

Other compatible patch updates and transitive resolutions are recorded in the lockfile and `npm-installed-after.json`. Direct vulnerable dependency minimums were raised, not merely the lock resolutions.

Vite 8 is a currently maintained line ([release policy](https://vite.dev/releases)); its [Node requirement](https://vite.dev/guide/) is now explicit in package.json: Node 20.19+ or 22.12+. Actual execution used Node 24.18.0. The existing release workflow selects Node 20 and was not changed or triggered. Vue I18n 9 is [end of life](https://vue-i18n.intlify.dev/guide/maintenance); version 11 retains this application's Composition API usage. ECharts 6 removes the affected older range; browser chart rendering is included in the separate E2E verification.

Vite 8 uses Rolldown, so the former object-form manualChunks configuration became an equivalent chunk-selection function. Element Plus now supplies generic table row types: explicit `@vue-generic` declarations preserve APIKey/S3Object/MigrateProgress types. Expired upload strings are mapped to named object rows for the table. No `any`, disabled type checks, or fast-only build substituted for strict verification.

## Reproduction and evidence

From `web/`:

```sh
npm ci
npm run build
npm audit --json
npm audit --omit=dev --json
npm run test:e2e
```

`npm ci` and strict `npm run build` (vue-tsc then Vite) passed. The build transcript is `docs/evidence/frontend-build-after.log`; E2E results belong to the browser verification report. Both final audit commands exited zero. The existing large-bundle warning remains visible (about 808 kB for the largest minified JavaScript chunk); it is not an audit vulnerability and has not been hidden by increasing thresholds. Local npm policy warned that optional fsevents and vue-demi lifecycle scripts were not approved; installation, type checking and bundling still completed successfully without changing that policy.
