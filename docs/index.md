---
title: Introduction
nav_order: 1
---

# Bast

**A structured Go framework for building efficient, scalable, and production server-side applications**

```bash
go get github.com/bastion-framework/bast@latest
```

Requires **Go 1.22+**.

---

## Philosophy

Bast is built on five principles that inform every design decision in the framework:

| Principle | What it means |
|-----------|---------------|
| **Explicit over implicit** | No magic. Every dependency is wired by hand. The framework never guesses what you want. |
| **Errors as values** | Handlers return `Response`. No writing to `http.ResponseWriter` directly, no panics, no side effects. |
| **Structure at scale** | Module-based organization enforced by the framework, not convention. Drop a module into a new project unchanged. |
| **Stdlib at the boundary** | Bast satisfies `http.Handler`. Every Go library, every middleware ecosystem, every deploy target works unchanged. |
| **Lean core** | Near-zero external dependencies. Every feature in core serves 80%+ of users. Everything else is a companion package. |

---

## What Bast gives you

- **Zero-alloc radix tree router** — `0 allocs/op` on every route match, including param routes
- **Pooled `*Ctx`** — `sync.Pool` based, 23 ns / 0 allocs per acquire+release
- **Module system** — self-contained units with explicit dependency injection at `main.go`
- **Typed error boundary** — one `ErrorHandler`, consistent JSON envelopes everywhere
- **Guards** — pre-handler checks that compose cleanly with OpenAPI security scheme generation
- **Built-in middleware** — `RequestID`, `Logger`, `Recover`, `CORS`
- **SSE / streaming** — `*StreamCtx`, never pooled, owns the wire
- **Health checks** — `/health` (liveness) and `/ready` (readiness) first-class, not route hacks
- **OpenAPI 3.0** — spec generated from code at startup, Swagger UI at `/docs`
- **Typed env config** — `LoadConfig[T]()` with struct tags, fails fast on boot
- **NestJS-style logger** — colored `[Bast]` boot and request logs, fully pluggable
- **`bast new` CLI** — scaffold a working Todo API in seconds
- **`basttest`** — unit and integration test helpers built into the framework

---

## Quick Start

```bash
# Install the CLI
go install github.com/bastion-framework/bast/cmd/bast@latest

# Scaffold a working project
bast new myapp
cd myapp

go mod tidy
go run .
```

Open [http://localhost:8080/docs](http://localhost:8080/docs) for the Swagger UI.  
Endpoints are live immediately — no configuration needed.

---

## What a Bast app looks like

```go
package main

import (
    "github.com/bastion-framework/bast"
    "github.com/bastion-framework/bast/middleware"

    "myapp/modules/users"
    "myapp/modules/auth"
)

func main() {
    app := bast.New(bast.Config{
        Port:         8080,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 30 * time.Second,
    })

    app.Use(
        middleware.RequestID,
        middleware.Recover,
        middleware.CORS(middleware.CORSConfig{
            AllowedOrigins: []string{"https://myapp.com"},
        }),
    )

    app.Register(
        auth.NewModule(pool),
        users.NewModule(pool),
    )

    app.Listen()
}
```

A handler:

```go
func (c *UsersController) GetUser(ctx *bast.Ctx) bast.Response {
    user, err := c.service.GetUser(ctx.Context(), ctx.Param("id"))
    if err != nil {
        return ctx.Error(err) // flows to the error boundary
    }
    return ctx.OK(user)
}
```

---

## Boot output

```
[Bast] ─────────────────────────────────────────────
[Bast] Bastion Framework v0.3.0
[Bast] ─────────────────────────────────────────────
[Bast] Module     Users               /users
[Bast]   GET      /users/             [AuthGuard]
[Bast]   GET      /users/:id          [AuthGuard]
[Bast]   POST     /users/             [AuthGuard]
[Bast] ─────────────────────────────────────────────
[Bast] Listening on :8080
[Bast] ─────────────────────────────────────────────
```

---

## Next steps

- [Getting Started](getting-started) — install, scaffold, run your first request
- [Core Concepts](concepts/modules) — understand the module system
- [CLI Reference](cli) — `bast new` and `bast generate`
- [Testing](features/testing) — unit and integration testing with `basttest`
- [Benchmarks](benchmarks) — performance numbers and methodology