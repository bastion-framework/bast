---
title: Benchmarks
nav_order: 6
---

# Benchmarks
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Methodology

All benchmarks run with:

```bash
go test -bench=. -benchmem -count=3 -run=^$ ./...
```

**Machine:** Intel Core i7-9700K @ 3.60GHz · Windows 11 · Go 1.25.4 amd64  
**Commit:** see `git log --oneline -1`

Benchmarks are run 3 times each; the table shows the median `ns/op`.

---

## Router

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| Static route match | 29 | 0 | **0** |
| Param route match (1 param) | 38 | 0 | **0** |
| Deep param route (5 params) | 93 | 0 | **0** |
| Not found | 64 | 0 | **0** |

Zero allocations across the board — including param routes. Route params are stored in a `[8]Param` array **embedded in the pooled `*Ctx` struct**, so the router writes directly into already-allocated memory. No map, no heap escape.

Param lookup (`ctx.Param(key)`) is a linear O(N) scan over the pre-allocated array. For N≤8, this is **faster than a map hash lookup** due to CPU cache locality.

---

## Ctx pool

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| Acquire + release | 21 | **0** | **0** |
| Param lookup | 3.5 | 0 | 0 |
| JSON body decode (`BindJSON`) | ~1860 | 2472 | 24 |

`BenchmarkCtx_AcquireRelease` measures the `sync.Pool` round-trip only — pre-created request and recorder outside the loop. The **0 allocs** result is a hard requirement; any PR that regresses this is blocked.

`BindJSON` allocations come from `encoding/json.Unmarshal`. This is unavoidable for JSON decoding and is not on the hot path if handlers early-return.

---

## Full request lifecycle

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| Static route, full lifecycle | 454 | 128 | 4 |
| Param route, full lifecycle | 473 | 128 | 4 |
| 5-middleware chain | 515 | 128 | 4 |
| Error boundary | 820 | 272 | 8 |

Static and param routes have **identical allocation counts** — route param extraction is free after the zero-alloc refactor.

The 4 remaining allocations on the happy path are from JSON serialization of the response envelope (`encoding/json.Marshal`) — unavoidable user-data work.

---

## Design decisions that drive these numbers

### `[8]Param` array in `*Ctx`

```go
type Ctx struct {
    paramStorage [8]Param     // allocated once with the Ctx, lives in the pool
    params       []Param      // slices into paramStorage
    // ...
}
```

The router receives `ctx.params` (backed by `ctx.paramStorage`) and appends into it. As long as the route has ≤8 params, all appends stay within the pre-allocated array — 0 heap escapes.

### Registration-time pipeline composition

```go
// Built once at registration time:
pipeline := buildPipeline(handler, allMiddleware)
// Wraps handlers via function composition — no slice iteration at request time
```

At request time, calling the pipeline is a single function call. No slice iteration, no interface dispatch overhead from the middleware list.

### `sync.Pool` for `*Ctx`

The pool `New` function pre-allocates the `store` map and zeroes all fields. `wipe()` resets slice length (not capacity) — the backing array is reused, never reallocated.

---

## Regression policy

| Benchmark | Requirement |
|-----------|-------------|
| `BenchmarkCtx_AcquireRelease` allocs | must be **0** — block merge |
| `BenchmarkRouter_*` allocs | must be **0** — block merge |
| `BenchmarkApp_*` ns/op | ≤ 5% regression requires justification |

```bash
# Check for regressions before merging:
go test -bench=. -benchmem -count=5 -run=^$ ./... > bench_head.txt
benchstat bench_base.txt bench_head.txt
```