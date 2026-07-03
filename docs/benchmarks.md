# Benchmarks

## Methodology

```bash
# Internal benchmarks
go test -bench=. -benchmem -count=3 -run=^$ ./...

# Comparative suite (separate module, tests the local checkout via replace)
cd bench && go test -bench=. -benchmem -benchtime=5s
```

**Machine:** Intel Core i7-9700K @ 3.60 GHz · Windows 11 · Go 1.25 amd64

All `net/http` frameworks are measured identically: `httptest.ResponseRecorder` +
`ServeHTTP`. Fiber (fasthttp) is excluded — its `app.Test()` harness pipes a full
HTTP/1.1 message in-process (~7 µs of overhead that is absent in production).

---

## Comparative: three tiers

### Tier 1 — Router lookup only (no HTTP stack)

| Benchmark | bast | httprouter |
|---|---:|---:|
| Static `GET /ping` | **13 ns · 0 allocs** | 12 ns · 0 allocs |
| Param `GET /users/:id` | **22 ns · 0 allocs** | 48 ns · 1 alloc |
| GitHub corpus (26 routes) | **32 ns · 0 allocs** | 59 ns · 0 allocs |

Bast's flat-arena router is ~2× faster than httprouter on param and corpus
lookups, and never allocates for path parameters.

### Tier 2 — Fair comparison (identical minimum-work handler)

| Framework | GitHub corpus | Static | allocs/op |
|---|---:|---:|---:|
| gin | 60 ns | 37 ns | 0 |
| httprouter | 67 ns | 20 ns | 0 |
| echo | 84 ns | 38 ns | 0 |
| **bast** | **139 ns** | **110 ns** | **0** |
| chi | 526 ns | 284 ns | 2–3 |
| gorilla/mux | 1 618 ns | 656 ns | 7 |

### Tier 3 — Full framework stack

Bast runs a realistic handler (`ctx.OK(nil)` — JSON envelope + `Content-Type`);
the others keep their minimal handler.

| Benchmark | bast | gin | echo | iris | stdlib | chi |
|---|---:|---:|---:|---:|---:|---:|
| GitHub corpus | **219 ns · 0 allocs** | 59 | 83 | 178 | 298 | 519 |
| Static | **177 ns · 0 allocs** | 37 | 42 | 83 | 92 | 284 |
| Param | **190 ns · 0 allocs** | 46 | 47 | 146 | 182 | 331 |

---

## Internal: router

| Benchmark | ns/op | allocs/op |
|-----------|------:|----------:|
| Static route match | 20 | **0** |
| Param route match (1 param) | 22 | **0** |
| Deep param route (5 params) | 86 | **0** |
| Not found | 23 | **0** |

Route params are written into a `[8]Param` array embedded in the pooled `*Ctx`
struct — the router appends into already-allocated memory. No map, no heap escape.

---

## Internal: Ctx and lifecycle

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| Ctx acquire + release | 20 | 0 | **0** |
| Param lookup | 3.3 | 0 | **0** |
| Static route, full lifecycle | 203 | 0 | **0** |
| Param route, full lifecycle | 204 | 0 | **0** |
| 5-middleware chain | 254 | 0 | **0** |
| Error boundary | 576 | 128 | 4 |

The full request lifecycle — router lookup, pooled Ctx, guard pipeline, JSON
envelope, `Content-Type` header, status, body write — is **zero allocations**.

---

## Design decisions that drive these numbers

**Flat-arena router** — the mutable insertion tree is BFS-frozen into a
contiguous `[]flatNode` on first lookup. Each node is two cache lines with hot
fields in the first; lookups walk a slice instead of chasing pointers.

**`[8]Param` in `*Ctx`** — the router appends params into storage that lives
inside the pooled struct. Zero heap escapes for ≤ 8 params.

**Registration-time pipeline composition** — middleware wraps handlers via
function composition at boot. Request time is a single function call.

**Pooled everything** — `*Ctx`, JSON envelopes, error envelopes, and request
body buffers (≤ 64 KB) all recycle through `sync.Pool`. The `store` map is
lazily allocated only for requests that actually use it.

**Header slice reuse** — `Content-Type` is written by reusing the existing
`[]string` in the header map, avoiding `Header.Set`'s per-call allocation.

**`go-json` backend** — JSON goes through the `internal/jsonx` seam wrapping
`github.com/goccy/go-json`; swapping backends is a one-line import change.

Every optimisation candidate is benchmarked before it ships — see the
[README optimisation ledger](https://github.com/bastion-framework/bast#benchmarks)
for what was rejected and why.

---

## Regression policy

| Benchmark | Requirement |
|-----------|-------------|
| `BenchmarkCtx_AcquireRelease` allocs | must be **0** — block merge |
| `BenchmarkRouter_*` allocs | must be **0** — block merge |
| `BenchmarkApp_StaticRoute` / `_ParamRoute` allocs | must be **0** — block merge |
| `BenchmarkApp_*` ns/op | ≤ 5% regression requires justification |

```bash
go test -bench=. -benchmem -count=5 -run=^$ ./... > bench_head.txt
benchstat bench_base.txt bench_head.txt
```
