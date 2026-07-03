# Modules

A module is the fundamental unit of organisation in a Bast application. It is **portable** and **self-contained** — it owns its repository, service, and controller entirely. Nothing leaks out except what it deliberately exposes.

---

## The Module struct

```go
type Module struct {
    Prefix     string
    Controller Controller
    Middleware  []MiddlewareFunc
    Guards     []Guard
    Modules    []Module     // nested sub-modules
    Doc        ModuleDoc    // OpenAPI tag group
}

type ModuleDoc struct {
    Name        string
    Description string
}
```

---

## Pattern A — Standalone module

Use when nothing else depends on this module's service:

```go
// modules/notifications/notifications.module.go
package notifications

func NewModule(db *pgxpool.Pool) bast.Module {
    repo       := newRepo(db)
    service    := newService(repo)
    controller := newController(service)

    return bast.Module{
        Prefix:     "/notifications",
        Controller: controller,
        Guards:     []bast.Guard{guards.AuthGuard},
        Doc: bast.ModuleDoc{
            Name:        "Notifications",
            Description: "Real-time notification management",
        },
    }
}
```

---

## Pattern B — Shared module

Use when other modules depend on this module's service. Embed `bast.Module` and expose `Service` as the public contract:

```go
// modules/users/users.module.go
package users

type Module struct {
    bast.Module          // embeds the framework module — registers routes
    Service *Service     // public contract — the ONLY thing siblings can touch
}

func NewModule(db *pgxpool.Pool) *Module {
    repo       := newRepo(db)
    service    := newService(repo)
    controller := newController(service)

    return &Module{
        Service: service,
        Module: bast.Module{
            Prefix:     "/users",
            Controller: controller,
            Guards:     []bast.Guard{guards.AuthGuard},
            Doc: bast.ModuleDoc{
                Name:        "Users",
                Description: "User management",
            },
        },
    }
}
```

A dependent module declares the dependency explicitly in its constructor:

```go
// modules/payments/payments.module.go
package payments

func NewModule(db *pgxpool.Pool, users *usersmod.Module) bast.Module {
    repo       := newRepo(db)
    service    := newService(repo, users.Service) // uses public API only
    controller := newController(service)

    return bast.Module{
        Prefix:     "/payments",
        Controller: controller,
        Guards:     []bast.Guard{guards.AuthGuard},
    }
}
```

---

## Composition root

`main.go` is the composition root. It is the one place that sees inter-module dependencies. This is intentional:

```go
func main() {
    pool := connectDB()

    // Shared modules first — they expose .Service
    usersModule := users.NewModule(pool)
    authModule  := auth.NewModule(pool)

    // Dependent modules receive the module, use .Service internally
    paymentsModule := payments.NewModule(pool, usersModule)
    profileModule  := profile.NewModule(pool, usersModule, authModule)

    app.Register(
        usersModule,
        authModule,
        paymentsModule,
        profileModule,
    )
    app.Listen()
}
```

`main.go` knowing inter-module dependencies is correct, not a flaw. Circular dependencies are impossible to compile, Go catches them before runtime.

---

## Nested modules (API versioning)

Modules can be nested. Child modules inherit the parent's prefix:

```go
v1 := bast.Module{
    Prefix: "/v1",
    Modules: []bast.Module{
        users.NewModule(pool),    // → /v1/users
        auth.NewModule(pool),     // → /v1/auth
        payments.NewModule(pool), // → /v1/payments
    },
}

v2 := bast.Module{
    Prefix: "/v2",
    Modules: []bast.Module{
        usersv2.NewModule(pool),  // → /v2/users (new version, separate package)
        auth.NewModule(pool),     // → /v2/auth  (same, reused)
    },
}

app.Register(v1, v2)
```

No versioning magic — just modules and prefixes.

---

## Module-scoped middleware and guards

Middleware and guards declared on a module apply to every route in that module. They run after global middleware/guards and before route-level middleware/guards:

```go
bast.Module{
    Prefix:     "/admin",
    Controller: adminController,
    Middleware: []bast.MiddlewareFunc{
        middleware.RateLimit(100), // applied to every /admin route
    },
    Guards: []bast.Guard{
        guards.AuthGuard,          // authentication
        guards.RequireRole("admin"), // authorisation
    },
}
```

Execution order is guaranteed:

```
Global Middleware → Global Guards →
Module Middleware → Module Guards →
Route Middleware  → Route Guards  →
Handler
```

---

## Rules

- Modules **never** import another module's `repo` or `controller` — only `Service`
- `Service` is the module's public API — keep it intentionally narrow
- `main.go` is the **only** place that wires inter-module dependencies
- Shared stateless utilities (JWT, hashing) go in `shared/services/` and are imported directly

---

## Portability

Because a module is a self-contained Go package, moving it to a new project is straightforward:

```bash
go get github.com/yourorg/users-module
```

```go
usersModule := users.NewModule(pool)
app.Register(usersModule)
```

The module brings its own routes, guards, service, and repo. You bring the database pool.
