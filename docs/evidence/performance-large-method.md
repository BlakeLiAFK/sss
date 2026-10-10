# 64 MiB concurrent-object benchmark

Executed on Apple M4 Pro, macOS arm64, Go 1.25.3. Baseline is `be66c1a53b6a51b71173ad67c11b386c3b3d1595`; current snapshot overlays the workspace's entire `internal/**/*.go` source on that archive. The exact Go source hashes are in `performance-large-snapshot-manifest.json`. The benchmark is the workspace `BenchmarkAuditConcurrentObjects`, restricted to the `64 << 20` size, with the independent-key regression test omitted from the baseline (it uses a new API absent there).

```sh
go test ./internal/api -run '^$' -bench '^BenchmarkAuditConcurrentObjects$' -benchtime=8x -count=3 -cpu=4
```

The snapshots were run sequentially with no simultaneous benchmark from this agent. Each measured run issues eight requests through the real server handler: seven PUTs of 64 MiB to distinct keys and one GET of a pre-existing small object. Go benchmark calibration also performs a one-operation warmup per repetition. All credentials and data are generated in temporary directories and cleaned by the test. This is in-process HTTP-handler concurrency, not network/TLS or a production load test.

| Metric | Before | After |
|---|---:|---:|
| Median ns/op | 122,246,292 | 97,749,406 |
| Observed ns/op range | 122,112,516–123,993,167 | 91,713,557–104,154,458 |
| Median allocated bytes/op | 58,818,358 | 58,818,560 |

For this bounded fixture, median elapsed time per operation decreased approximately 20.0%. There are only three repetitions and eight measured requests per repetition; machine contention and filesystem cache effects are not controlled, so this is directional evidence, not a capacity guarantee. Allocation remains essentially unchanged; payload generation and signing helpers contribute to it.

The raw Go MB/s column uses `SetBytes(64 MiB)` on every operation, including the small GET. It overstates actual bytes transferred and must not be described as measured network throughput. Prefer the ns/op comparison. Raw passing output is preserved in `performance-large-before.txt` and `performance-large-after.txt`.
