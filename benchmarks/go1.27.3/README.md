# Modifier allocation optimization

Measured on 2026-10-23 with Go 1.27.3, darwin/arm64, Apple M3 Pro.
Both versions use the same installed compiler. The baseline uses the original
`go 1.27.0` directive and production code; the candidate uses `go 1.27.3`.
The same benchmark and compatibility tests were run against both versions.

`Flatten` now appends leaves into a shared result accumulator instead of
allocating intermediate results for each nested collection and nil leaf.
It still returns an independent slice, preserves typed and untyped nil values,
and returns nil for collections with no leaves. `TupleNames` reserves space
for parentheses, nonempty names, and separators before writing; empty names
still disappear and an empty tuple returns `()` directly.

| Workload | B/op before → after | allocs/op before → after |
| --- | ---: | ---: |
| Flatten empty | 0 → 0 | 0 → 0 |
| Flatten flat, including nil | 256 → 240 | 5 → 4 |
| Flatten nested, including nil | 528 → 240 | 13 → 4 |
| TupleNames | 56 → 32 | 2 → 1 |

Samples are retained in [before.txt](before.txt) and [after.txt](after.txt).
Runs were sequential, five samples each, with default GC settings:

```sh
go test -run '^$' -bench 'Benchmark(Flatten|TupleNames)$' -benchmem -benchtime=200ms -count=5
```

These are allocation measurements for the listed inputs, not a guarantee of
zero allocation for arbitrary queries. Timings are noisy and no statistical
runtime speedup is claimed. Returning independent nonempty slices and generated
SQL strings still requires allocation. Empty Flatten and TupleNames paths have
an explicit zero-allocation test; literal compilation already has a zero-allocation
fast path.

Validation: `go vet ./...`, `go test -race -cover ./...` (96.3% statement coverage),
`gofmt`, and `git diff --check`. Compatibility tests cover empty names, Unicode,
arrays, nested collections, nil values, and output ownership, and also passed on
the baseline. `betteralign -apply ./...` was reviewed and all layout rewrites
were reverted to preserve field order and avoid unrelated changes. A final
`betteralign ./...` still reports 15 existing layout diagnostics, including
public builder types and the internal named argument wrapper.
