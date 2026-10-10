# GC memory measurements

Measured on macOS arm64 (Darwin 25.5.0), Go 1.25.3. Baseline is `be66c1a53b6a51b71173ad67c11b386c3b3d1595`. The optimized snapshot consists of that archive with the current `internal/storage/*.go` overlaid. `snapshot-manifest.json` records SHA-256 hashes of both snapshots. Only `/tmp` snapshots received the diagnostic test; production code was not changed for measurement.

## Method

`probe_test.go.txt` is the exact test. Copy it to `internal/storage/gc_memory_evidence_test.go` in each snapshot, then run sequentially:

```sh
go test ./internal/storage -run '^TestGCMemoryEvidence$' -count=1 -v
```

Each isolated fixture has 20,000 or 80,000 one-byte files using the normal hashed directory layout, with metadata for 95% and 5% orphans. Fixture creation is outside measurement. Each mode runs the same dry-run `RunGC` three times, verifying the full orphan count. The original benchmark uses the same layout and ratio. At 80,000 files the original returns all 4,000 orphan details; the optimized implementation intentionally retains a 1,000-entry response sample while counting all 4,000.

- Before each run, `runtime.GC()` establishes a collected baseline.
- **natural-occupancy** samples `runtime.MemStats.HeapAlloc` every 2ms with normal Go GC behavior. This measures occupied Go heap, including garbage not yet collected; it is not live heap.
- **forced-gc-live** is a separate diagnostic. Every 20ms it explicitly runs GC then samples HeapAlloc, approximating retained/live Go heap during the scan. The mutator can allocate between collection and sampling. This mode perturbs timing and GC behavior and must not be used as a throughput benchmark.
- Peak increments subtract the pre-run HeapAlloc. They are sampled maxima, not exact peaks or guaranteed upper bounds. In the shortest baseline run there are only three forced-GC samples.
- `TotalAlloc` is separately recorded and is cumulative allocation, not peak or retained memory. The result is kept alive through an end-of-run GC to measure retained output as well.
- Runs are sequential, not randomized, and share a machine with other work. Filesystem/cache effects and machine contention are uncontrolled. This is bounded evidence of scaling for this fixture, not a universal constant-memory or latency guarantee.
- These measurements exclude native allocations, SQLite/filesystem caches and process RSS. They do not exercise huge directory depth, millions of uploads, deletion mode or large file contents. Streaming directory depth and sample-size bounds still matter.

## Results

Values below are median peak **increments** in MiB, with observed min–max over three runs.

| Files | Measurement | Before MiB | After MiB |
|---:|---|---:|---:|
| 20,000 | natural-occupancy | 13.794 (13.573–13.802) | 2.903 (2.805–2.936) |
| 20,000 | forced-gc-live | 5.859 (5.447–5.943) | 0.364 (0.360–0.389) |
| 80,000 | natural-occupancy | 51.158 (50.861–51.350) | 3.052 (3.030–3.611) |
| 80,000 | forced-gc-live | 27.356 (27.306–27.487) | 0.447 (0.418–0.460) |

The original sampled heap increases substantially when the fixture grows fourfold; the optimized sampled heap changes little. Both the unforced occupancy experiment and the separate post-collection experiment support removal of the whole-store retained object/path collection for this workload. The data do not establish a strict memory ceiling.

Cumulative allocations remain proportional to work: at 20,000 files, median natural-mode total allocations fell from 34.25 to 22.76 MiB; at 80,000, from 138.64 to 88.02 MiB. This is a different metric from the bounded sampled retained footprint.

There is a time tradeoff: median natural-mode elapsed time was 79.1 → 107.4ms at 20,000 and 316.7 → 436.9ms at 80,000 (approximately 36–38% slower in this probe). The indexed batch membership queries exchange some scan speed for much lower retained memory. Use the independent benchmark logs for throughput reporting; do not present this optimization as faster GC.

Both probe executions passed. `before.log` and `after.log` are raw test output; `measurements.json` preserves every record. These artifacts contain only generated fixture paths/data.
