# BENCH.md — Bast Benchmark Baselines

> Machine: Intel Core i7-9700K @ 3.60GHz · Windows 11 · Go 1.25.4 amd64
> Measured with: `go test -bench=. -benchmem -count=3 -run=^$`
> Date: 2026-06-08 · Commit: see `git log --oneline -1`

Regressions above **5%** on any `BenchmarkApp_*` require justification in the PR.
The `BenchmarkCtx_AcquireRelease` number **must remain 0 allocs/op**.

---

## Router — `github.com/bastion-framework/bast/router`

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkRouter_Static` | 66 | 48 | 1 |
| `BenchmarkRouter_Param` | 185 | 336 | 2 |
| `BenchmarkRouter_DeepParam` (5 params) | 290 | 336 | 2 |
| `BenchmarkRouter_NotFound` | 105 | 48 | 1 |

The 1 allocation on static routes and the 2 on param routes are from the `Match`
struct returned by `Find` (the `Params` map). This is expected and unavoidable
without an arena allocator. The map is pre-sized to 8 keys at pool creation time —
no growth occurs on typical routes.

---

## Ctx — `github.com/bastion-framework/bast`

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkCtx_AcquireRelease` | 23 | **0** | **0** |
| `BenchmarkCtx_ParamLookup` | 9 | 0 | 0 |
| `BenchmarkCtx_Bind` (JSON decode) | 1860 | 2232 | 25 |

`BenchmarkCtx_AcquireRelease` measures the pool round-trip only (`Get` + field
assignment + `wipe` + `Put`). The request and recorder are pre-created outside
the loop. **0 allocs/op is a hard requirement** — any PR that breaks this must
be rejected until fixed.

`BenchmarkCtx_Bind` allocations come from `json.Unmarshal` and are unavoidable
for JSON decoding. This is not on the hot path if the handler short-circuits early.

---

## App (full request lifecycle) — `github.com/bastion-framework/bast`

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkApp_StaticRoute` | 499 | 176 | 5 |
| `BenchmarkApp_ParamRoute` | 690 | 464 | 6 |
| `BenchmarkApp_MiddlewareChain` (5 noop MW) | 546 | 176 | 5 |
| `BenchmarkApp_ErrorBoundary` | 924 | 320 | 9 |

The 5 allocations on a static route come from:
1. `httptest.ResponseRecorder` write buffer growth (test harness overhead)
2. JSON marshal of the response envelope (`json.Marshal`)
3. `map[string]any` in the response envelope

In production with a real `http.ResponseWriter`, items 1 is gone.
Items 2–3 are on the response serialization path — not the routing/dispatch path.

---

## Regression thresholds

| Scope | Threshold | Action |
|---|---|---|
| `BenchmarkCtx_AcquireRelease` allocs | must be **0** | Block merge |
| `BenchmarkApp_*` ns/op | ≤ 5% regression | Require justification |
| `BenchmarkRouter_*` ns/op | ≤ 10% regression | Flag in review |

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