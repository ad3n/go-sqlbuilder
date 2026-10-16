# Go 1.27 allocation optimization

Measured on 2026-10-16 using Go 1.27.2, linux/amd64 (GOAMD64=v1), AMD Ryzen AI 9 HX 375. Baseline commit: `85193a6f2e28a320fad4523660352dbef7de74ed`.

Both versions use the same Go 1.27.2 compiler. The baseline retains its original `go 1.26.0` module directive; the candidate uses `go 1.27.0`. This comparison measures the code changes and does not establish a speedup from upgrading the compiler.

The retained optimization reserves the intermediate SQL buffer for INSERTs with 2–1024 rows whose estimated template fits within 64 KiB, using placeholder lengths, row separators, and the repeated Oracle header. Normal growth still handles injected SQL, RETURNING, and other additional text. Single-row INSERTs and INSERT SELECT skip the estimation pass. Larger row counts skip estimation, bounding the extra scan work. Templates estimated above 64 KiB keep the previous growth behavior: both full reservation and a capped reservation regressed the PostgreSQL 5000-row lifecycle benchmark, so those variants were rejected. Returned SQL and argument slices retain their existing ownership; no additional result pooling is introduced.

Embedding `strings.Builder` in its internal wrapper leaves allocation counts unchanged but reduces pointer indirection. A CPU-pinned, ten-sample control comparison measured approximately 4–8% lower time for the PostgreSQL/Oracle SELECT and single-row INSERT workloads. `betteralign -apply ./...` was run; its field reorderings were discarded to preserve existing layouts and avoid unrelated, unmeasured changes.

## Method

`before.txt` and `after.txt` contain ten samples per case, collected from separately compiled binaries. Each sample uses:

```sh
taskset -c 0 go test -run '^$' -bench 'Benchmark(Scan|InsertLifecycle)' -benchmem -cpu=1 -benchtime=200ms -count=1
```

Baseline and candidate runs execute sequentially on CPU 0 (`taskset -c 0`), alternating which version goes first on each of ten rounds. Benchmarks do not run concurrently. `comparison.txt` is the complete benchstat comparison, using `golang.org/x/perf` version `v0.0.0-20260929162123-406019bb8b68` and its default significance threshold of 0.05.

To reproduce the baseline, export the baseline commit into a separate directory and copy `insert_optimization_test.go` into it before compiling. This adds the same lifecycle benchmarks to both versions without modifying baseline production code. `BenchmarkScan` repeatedly builds an already populated builder; `BenchmarkInsertLifecycle` includes creation, Values calls, and Build for 2, 10, 1000, and 5000 rows. The latter includes SQL larger than the 64 KiB pooled compile-buffer limit. Benchmark outputs are retained in package-level sinks.

The results describe these workloads on this machine. Lower B/op and allocs/op measure reduced allocation traffic; they do not by themselves measure GC pause duration or production latency. Default GC settings are used.

## Validation

The additional compatibility tests also pass against the baseline. They cover nested builders, raw expressions, duplicate named arguments, initial arguments, `SQL()` fragments, RETURNING/OUTPUT, Clone, differing stored/output flavors, INSERT SELECT precedence, and retained results exceeding 64 KiB. Existing tests cover all supported SQL flavors, placeholder boundaries, ownership, pool cleanup, and concurrent compilation.

## Results

Across the 26 workloads, benchstat reports no significant increases in time, B/op, or allocs/op at alpha 0.05. Literal compilation remains allocation-free. Representative 1000-row results (medians):

| Workload | KiB/op before → after | Allocs/op before → after | Time before → after | Time change |
| --- | ---: | ---: | ---: | ---: |
| Build only, MySQL | 277.9 → 190.4 | 34 → 16 | 121.9 → 110.9 µs | -9.05% |
| Build only, PostgreSQL | 289.9 → 202.4 | 34 → 16 | 164.9 → 150.8 µs | -8.59% |
| Build only, Oracle | 433.8 → 282.5 | 32 → 16 | 232.2 → 188.4 µs | -18.85% |
| Create + Values + Build, MySQL | 562.6 → 475.2 | 4744 → 4726 | 232.6 → 211.0 µs | -9.26% |
| Create + Values + Build, PostgreSQL | 574.6 → 487.1 | 4744 → 4726 | 269.8 → 244.6 µs | -9.31% |
| Create + Values + Build, Oracle | 718.5 → 567.1 | 4742 → 4726 | 332.1 → 284.6 µs | -14.29% |

Full samples: [before.txt](before.txt), [after.txt](after.txt). Full statistical comparison: [comparison.txt](comparison.txt).

## GC collections under fixed work

Each version executed exactly 5000 iterations per workload, three times, alternating order, on CPU 0 with `GOMAXPROCS=1`, `GOGC=100`, `GOMEMLIMIT=off`, and `GODEBUG=gctrace=1`. Natural collections were counted from runtime trace lines beginning with `gc`; lines containing `(forced)` were excluded. These runs measure collection counts, not pause durations or latency; trace logging overhead is not used for timing comparisons.

| MySQL, 1000 rows | Natural GC cycles before → after, median | Change |
| --- | ---: | ---: |
| Build only | 535 → 346 | -35.33% |
| Create + Values + Build | 1065 → 853 | -19.91% |

All individual counts, including excluded forced collections: [gc-collections.tsv](gc-collections.tsv).

Example fixed-work invocation, repeated for each version:

```sh
GOMAXPROCS=1 GOGC=100 GOMEMLIMIT=off GODEBUG=gctrace=1 \
  taskset -c 0 go test -run '^$' \
  -bench '^BenchmarkInsertLifecycle/MySQL/Rows1000$' \
  -benchmem -cpu=1 -benchtime=5000x -count=1 2>gc.trace
```

Final validation: `go test -race -cover ./...` passed with 96.2% statement coverage; `go vet ./...` passed. Each of `FuzzWriteStringsPrefixed`, `FuzzWriteInsertRow`, `FuzzCompilePoolLifetime`, `FuzzSelect`, and `FuzzSelectClone` passed a five-second fuzz run with two workers. `goimports`, `gofmt`, and `git diff --check` passed. Build/test caches were directed to `/tmp/go-sqlbuilder-gocache` because the default cache is read-only in this environment.
