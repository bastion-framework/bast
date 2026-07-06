# CLI — `bast`

## Installation

```bash
go install github.com/bastion-framework/bast/cmd/bast@latest
```

---

## `bast new`

Scaffolds a new Bast project with a fully working **Todo API**.

```bash
bast new <appname>
```

```bash
bast new myapp
# ✓ Created project myapp
#   cd myapp && go mod tidy && go run .
```

### Generated structure

```
myapp/
├── main.go
├── go.mod
├── modules/
│   └── todos/
│       ├── todos.module.go
│       ├── todos.controller.go     # full CRUD with OpenAPI Doc{}
│       ├── todos.service.go
│       ├── todos.repo.go           # in-memory store, mutex-safe
│       └── todos.dto.go
└── shared/
    ├── guards/
    │   └── auth.guard.go
    └── errors/
        └── errors.go
```

The app works immediately after `go mod tidy && go run .` — no database, no configuration needed.

### What's included out of the box

| Endpoint | Description |
|----------|-------------|
| `GET /todos/` | List all todos |
| `GET /todos/:id` | Get one todo |
| `POST /todos/` | Create a todo |
| `PATCH /todos/:id` | Update title / mark done |
| `DELETE /todos/:id` | Delete a todo |
| `GET /health` | Liveness probe |
| `GET /ready` | Readiness probe |
| `GET /docs` | Swagger UI |
| `GET /openapi.json` | Raw OpenAPI 3.0.3 spec |

---

## `bast generate module`

Generates the 5 files that make up a Bast module inside `modules/<name>/` and **automatically registers** it in `main.go`.

```bash
bast generate module <name>
# Aliases: gen, g
```

```bash
bast generate module payments
# ✓ Generated module/payments → modules/payments
#   ↳ registered in main.go
```

### Generated files

```
modules/payments/
├── payments.module.go
├── payments.controller.go    # full CRUD routes
├── payments.service.go       # business logic layer
├── payments.repo.go          # in-memory repo stub
└── payments.dto.go           # Payment, CreateRequest, UpdateRequest
```

### What's auto-added to `main.go`

```go
// Import added:
import "myapp/modules/payments"

// Registration added:
app.Register(
    todos.NewModule(),
    payments.NewModule(), // ← added
)
```

If `main.go` cannot be found or doesn't follow the standard pattern, the CLI prints a warning and tells you exactly what to add manually.

---

## `bast generate guard`

Generates a guard in `shared/guards/`.

```bash
bast generate guard <name>
```

```bash
bast generate guard admin
# ✓ Generated guard/admin → shared/guards
```

Generated file (`shared/guards/admin.guard.go`):

```go
package guards

import "github.com/bastion-framework/bast"

var AdminGuard = bast.GuardFunc(func(ctx *bast.Ctx) error {
    // Return nil to allow, return an error to block.
    // Example: check a role stored by a prior auth guard.
    // claims, ok := bast.Get[*Claims](ctx, "claims")
    // if !ok || claims.Role != "admin" {
    //     return bast.ErrForbidden(bast.CodeForbidden, "insufficient role")
    // }
    return nil
})
```

---

## `bast generate service`

Generates a shared service in `shared/services/`.

```bash
bast generate service <name>
```

```bash
bast generate service token
# ✓ Generated service/token → shared/services
```

Use shared services for stateless utilities (JWT, hashing, email) that multiple modules depend on.

---

## `bast run`

Runs the app in the current project directory.

```bash
bast run            # equivalent to go run . with graceful Ctrl+C handling
bast run --watch    # rebuild + restart whenever .go files, go.mod, or go.sum change
```

Watch mode compiles to a temp binary and restarts the process on every change —
no third-party watcher needed. `vendor/`, `bin/`, `dist/`, and dot-directories
are ignored. If a rebuild fails, the previous process keeps running until the
code compiles again.

---

## `bast build`

Produces a production-ready binary.

```bash
bast build                          # → bin/<app> (module name)
bast build -o dist/api              # custom output path
bast build --os linux --arch arm64  # cross-compile
```

```bash
bast build
# ✓ Built bin/myapp (6.8 MB)
```

Every build is:

| Flag | Effect |
|------|--------|
| `-trimpath` | Reproducible — no local filesystem paths embedded |
| `-ldflags "-s -w"` | Stripped symbol/debug tables — smaller binaries |
| `CGO_ENABLED=0` | Statically linked — runs in `scratch` / distroless containers |

---

## Command aliases

```bash
bast generate module payments
bast gen module payments       # alias
bast g module payments         # alias
```

---

## Help

```bash
bast help
bast --help
bast -h
```
