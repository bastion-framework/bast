<p align="center">
  <img src="https://raw.githubusercontent.com/bastion-framework/bast/main/bast.png" width="200" alt="Bast Logo" />
</p>

<h1 align="center">Bast</h1>

<p align="center">
 A structured Go framework for building efficient, scalable, and production server-side applications.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/bastion-framework/bast"><img src="https://pkg.go.dev/badge/github.com/bastion-framework/bast.svg" alt="Go Reference" /></a>
  <a href="https://github.com/bastion-framework/bast/actions"><img src="https://github.com/bastion-framework/bast/workflows/CI/badge.svg" alt="CI" /></a>
  <img src="https://img.shields.io/badge/go-%3E%3D1.22-blue" alt="Go version" />
  <img src="https://img.shields.io/github/license/bastion-framework/bast" alt="License" />
  <img src="https://img.shields.io/badge/version-v0.1.0-green" alt="Version" />
</p>

---

## Philosophy

| | |
|---|---|
| **Explicit over implicit** | No magic. Every dependency is wired by hand. |
| **Errors as values** | Handlers return `Response`. No side effects, no panic. |
| **Structure at scale** | Module-based organization. Enforced by the framework. |
| **Stdlib at the boundary** | Bast satisfies `http.Handler`. The full Go ecosystem works. |
| **Lean core** | Near-zero dependencies. Batteries are opt-in companion packages. |

---

## Features

- **Radix tree router** — zero allocations on the hot path, static-wins-over-param priority
- **Pooled `*Ctx`** — `sync.Pool` based, 0 allocs per request for routing and context
- **Module system** — portable, self-contained units with dependency injection at `main.go`
- **Typed error boundary** — one `ErrorHandler`, consistent JSON envelopes everywhere
- **Built-in middleware** — `RequestID`, `Logger`, `Recover`, `CORS`
- **Guards** — pre-handler checks with `SecuredGuard` → OpenAPI security scheme auto-wiring
- **Streaming** — `*StreamCtx` for SSE and chunked responses, never pooled
- **Health checks** — `/health` (liveness) and `/ready` (readiness) with dependency checks
- **OpenAPI 3.0** — spec generated from code at startup; Swagger UI at `/docs`
- **Typed env config** — `LoadConfig[T]()` with `env`, `default`, `required`, `secret` tags
- **Logger** — colored `[Bast]` boot and request logs, fully pluggable
- **`bast new` CLI** — scaffold a working Todo API in seconds
- **`basttest`** — unit and integration test helpers built into the framework

---

## Quick Start

```bash
# Install the CLI
go install github.com/bastion-framework/bast/cmd/bast@latest

# Scaffold a new project (generates a working Todo API)
bast new myapp
cd myapp

# Wire the local framework and run
go mod tidy
go run .
```

Open [http://localhost:8080/docs](http://localhost:8080/docs) for the Swagger UI.

---

## Installation

```bash
go get github.com/bastion-framework/bast@latest
```

Requires **Go 1.22+**.


---

## CLI

```bash
bast new <appname>                  # Scaffold a new project
bast generate module <name>         # Generate a module (5 files)
bast generate guard  <name>         # Generate a guard
bast generate service <name>        # Generate a shared service
```

---

## Testing

`basttest` ships with the framework — no separate install needed.

```go
// Unit test — handler as a pure function
func TestGetUser(t *testing.T) {
    ctx := basttest.NewCtx(
        basttest.WithParam("id", "42"),
        basttest.WithStore("claims", &Claims{Role: "admin"}),
    )
    resp := controller.GetUser(ctx)
    assert.Equal(t, 200, resp.Status())
}

// Integration test — full request lifecycle
func TestCreateUser(t *testing.T) {
    app := basttest.NewApp(users.NewModule(testDB))
    app.Do("POST", "/users",
        basttest.WithJSONBody(CreateUserRequest{Name: "Kasim", Email: "k@suds.ug"}),
    ).Assert(t).StatusIs(201).BodyContains("id")
}
```

---

## Benchmarks

Intel i7-9700K @ 3.60 GHz, Go 1.25, windows/amd64. Three benchmark tiers — run them all with:

```
cd bench && go test -bench=. -benchmem -benchtime=5s
```

---

### Tier 1 — Router lookup only (no HTTP stack)

Measures pure route-matching time by calling the router's lookup API directly.
Bast's flat-arena BFS layout outperforms httprouter on non-trivial routes — and never allocates for path parameters.

| Benchmark | bast | httprouter |
|---|---:|---:|
| Static `GET /ping` | **13 ns · 0 allocs** | 12 ns · 0 allocs |
| Param `GET /users/:id` | **22 ns · 0 allocs** | 48 ns · 1 alloc |
| GitHub corpus (26 routes, 8 requests) | **32 ns · 0 allocs** | 59 ns · 0 allocs |

---

### Tier 2 — Fair comparison (all frameworks, minimum-work handler)

Every framework returns status 200 with no body and no extra headers — the same workload gin and echo use in their own published benchmarks.
This isolates routing + dispatch from response-writing.

#### GitHub API corpus

| Framework | ns/op | allocs/op |
|---|---:|---:|
| gin | 60 | 0 |
| httprouter | 67 | 0 |
| echo | 84 | 0 |
| **bast** | **139** | **0** |
| chi | 526 | 3 |
| gorilla/mux | 1 618 | 7 |

#### Static route — `GET /ping`

| Framework | ns/op | allocs/op |
|---|---:|---:|
| httprouter | 20 | 0 |
| gin | 37 | 0 |
| echo | 38 | 0 |
| **bast** | **110** | **0** |
| chi | 284 | 2 |
| gorilla/mux | 656 | 7 |

---

### Tier 3 — Full framework stack

Bast uses a realistic handler (`ctx.OK(nil)`) that writes a proper JSON envelope with `Content-Type: application/json`. Other frameworks still use their minimum-work handler.

| Benchmark | bast | gin | echo | httprouter | iris | stdlib | chi | gorilla/mux |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| GitHub corpus | **219 ns · 0 allocs** | 59 | 83 | 66 | 178 | 298 | 519 | 1 548 |
| Static | **177 ns · 0 allocs** | 37 | 42 | 20 | 83 | 92 | 284 | 668 |
| Param | **190 ns · 0 allocs** | 46 | 47 | 56 | 146 | 182 | 331 | 874 |

> Fiber (fasthttp) is excluded — its `app.Test()` harness pipes a full HTTP/1.1 message in-process (~7 µs overhead absent in production).

---

**Further headroom** — not yet implemented, each shaves additional allocations or latency:

- **Request body buffer pooling** — pool the `bytes.Buffer` in `readBody()` to eliminate 1 alloc on any body-reading endpoint
- **Lazy `Ctx.store` init** — skip the `map[string]any` allocation for routes that never call `ctx.Set()`
- **Route-level content-type hint** — embed the content type in the route descriptor so `writeResponse` can skip the header write entirely for static routes
- **Per-shard `sync.Pool`** — NUMA-aware per-P pool sharding eliminates cross-core cache invalidation at high concurrency
- **Arena allocation** — monotonic per-request arena wiped as a single `runtime.MemClr`, eliminating GC pressure for all per-request objects

---

## Companion packages

| Package | Description |
|---|---|
| `github.com/bastion-framework/bast-pgx` | pgx/pgxpool repo helpers, transaction utilities |
| `github.com/bastion-framework/bast-gorm` | Community maintained |
| `github.com/bastion-framework/bast-sqlc` | Community maintained |

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

---

## License

Bast is [MIT licensed](LICENSE).

---

<p align="center">Author <a href="https://github.com/kasimlyee">Kasim Lyee</a></p>