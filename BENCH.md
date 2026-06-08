# BENCH.md — Bast Benchmark Baselines

> Machine: Intel Core i7-9700K @ 3.60GHz · Windows 11 · Go 1.25.4 amd64
> Measured with: `go test -bench=. -benchmem -count=3 -run=^$`
> Date: 2026-06-08 · Commit: see `git log --oneline -1`

Regressions above **5%** on any `BenchmarkApp_*` require justification in the PR.
All router and Ctx pool benchmarks **must remain at 0 allocs/op**.

---

## Router — `github.com/bastion-framework/bast/router`

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkRouter_Static` | 29 | **0** | **0** |
| `BenchmarkRouter_Param` | 38 | **0** | **0** |
| `BenchmarkRouter_DeepParam` (5 params) | 93 | **0** | **0** |
| `BenchmarkRouter_NotFound` | 64 | **0** | **0** |

Zero allocations achieved by writing route params directly into a pre-allocated
`[8]Param` array embedded in `Ctx`. The router receives a caller-provided `[]Param`
slice backed by that array — no map, no heap allocation on any code path.

Param lookup (`Ctx.Param`) is a linear O(N) scan over the slice.
For N≤8 this is faster than a hash lookup due to CPU cache locality.

---

## Ctx — `github.com/bastion-framework/bast`

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkCtx_AcquireRelease` | 21 | **0** | **0** |
| `BenchmarkCtx_ParamLookup` | 3.5 | **0** | **0** |
| `BenchmarkCtx_Bind` (JSON decode) | 1860 | 2472 | 24 |

`BenchmarkCtx_ParamLookup` dropped from 9 ns → 3.5 ns after the map → slice refactor
(2.6× faster: linear scan beats hash for small N due to cache locality).

`BenchmarkCtx_Bind` allocations are from `json.Unmarshal` — unavoidable for JSON decoding.

---

## App (full request lifecycle) — `github.com/bastion-framework/bast`

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkApp_StaticRoute` | 454 | 128 | 4 |
| `BenchmarkApp_ParamRoute` | 473 | 128 | 4 |
| `BenchmarkApp_MiddlewareChain` (5 noop MW) | 515 | 128 | 4 |
| `BenchmarkApp_ErrorBoundary` | 820 | 272 | 8 |

Static and param routes now have **identical allocation counts (4)** — the param extraction
is free. The remaining 4 allocations on the happy path are from response JSON serialization
(`json.Marshal` of the response envelope), which is unavoidable user-data work.

---

## Regression thresholds

| Scope | Threshold | Action |
|---|---|---|
| `BenchmarkCtx_AcquireRelease` allocs | must be **0** | Block merge |
| `BenchmarkRouter_*` allocs | must be **0** | Block merge |
| `BenchmarkApp_*` ns/op | ≤ 5% regression | Require justification |

## How to check for regressions

```bash
# Before your change — save baseline
go test -bench=. -benchmem -count=5 -run=^$ ./... > bench_base.txt

# After your change
go test -bench=. -benchmem -count=5 -run=^$ ./... > bench_head.txt

# Compare
go install golang.org/x/perf/cmd/benchstat@latest
benchstat bench_base.txt bench_head.txt
```