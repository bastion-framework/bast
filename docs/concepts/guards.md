---
title: Guards
parent: Concepts
nav_order: 4
---

# Guards
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

Guards run **before** the handler. Return `nil` to allow the request through; return an error to block it. Guards are cleaner than middleware for authentication and authorisation because the intent is explicit and named.

---

## The Guard interface

```go
type Guard interface {
    Check(ctx *Ctx) error
}
```

## GuardFunc — function adapter

For simple guards that don't need state:

```go
type GuardFunc func(ctx *Ctx) error

func (f GuardFunc) Check(ctx *Ctx) error { return f(ctx) }
```

---

## Writing a guard

### JWT authentication guard

```go
var AuthGuard = bast.GuardFunc(func(ctx *bast.Ctx) error {
    token := ctx.Header("Authorization")
    if token == "" {
        return bast.ErrUnauthorized(bast.CodeUnauthorized, "missing Authorization header")
    }

    claims, err := jwt.Verify(strings.TrimPrefix(token, "Bearer "))
    if err != nil {
        return bast.ErrUnauthorized(bast.CodeUnauthorized, "invalid or expired token")
    }

    ctx.Set("claims", claims)
    return nil
})
```

### Role guard factory

```go
func RequireRole(role string) bast.Guard {
    return bast.GuardFunc(func(ctx *bast.Ctx) error {
        claims, ok := bast.Get[*Claims](ctx, "claims")
        if !ok {
            return bast.ErrForbidden(bast.CodeForbidden, "no claims in context")
        }
        if claims.Role != role {
            return bast.ErrForbidden(bast.CodeForbidden, "requires role: "+role)
        }
        return nil
    })
}

// Usage:
bast.DELETE("/:id", c.Delete,
    bast.WithGuards(RequireRole("admin")),
)
```

---

## Registering guards

Guards can be registered at three levels. They run in this order:

```
Global Guards → Module Guards → Route Guards
```

### Global — runs on every request

```go
app.Guard(guards.AuthGuard)
```

### Module — runs on every route in the module

```go
bast.Module{
    Prefix:     "/admin",
    Controller: adminController,
    Guards: []bast.Guard{
        guards.AuthGuard,
        guards.RequireRole("admin"),
    },
}
```

### Route — runs on that route only

```go
bast.DELETE("/:id", c.Delete,
    bast.WithGuards(guards.RequireRole("admin")),
)
```

---

## SecuredGuard — OpenAPI integration

Guards that implement `SecuredGuard` automatically populate the `securitySchemes` section of the generated OpenAPI spec. Swagger UI will show the lock icon and prompt for credentials.

```go
type SecuredGuard interface {
    Guard
    SecurityScheme() bast.SecurityScheme
}

type SecurityScheme struct {
    Type         string // "http", "apiKey", "oauth2"
    Scheme       string // "bearer", "basic"
    BearerFormat string // "JWT"
    Description  string
}
```

### Example — JWT guard with OpenAPI integration

```go
type jwtGuard struct{}

func (g *jwtGuard) Check(ctx *bast.Ctx) error {
    token := ctx.Header("Authorization")
    if token == "" {
        return bast.ErrUnauthorized(bast.CodeUnauthorized, "missing token")
    }
    claims, err := jwt.Verify(strings.TrimPrefix(token, "Bearer "))
    if err != nil {
        return bast.ErrUnauthorized(bast.CodeUnauthorized, "invalid token")
    }
    ctx.Set("claims", claims)
    return nil
}

func (g *jwtGuard) SecurityScheme() bast.SecurityScheme {
    return bast.SecurityScheme{
        Type:         "http",
        Scheme:       "bearer",
        BearerFormat: "JWT",
        Description:  "JWT access token obtained from POST /auth/login",
    }
}

// Declare as a pointer so it satisfies SecuredGuard
var AuthGuard bast.Guard = &jwtGuard{}
```

When `AuthGuard` is registered on a module, Bast automatically marks all routes in that module as secured in the spec and adds `bearerAuth` to `securitySchemes`.

---

## Guard vs Middleware

| | Guard | Middleware |
|--|-------|-----------|
| Intent | Allow or block | Transform request/response |
| Return on block | `error` | `Response` (short-circuit) |
| Named | Yes — type name appears in logs and OpenAPI | No |
| OpenAPI integration | Yes — via `SecuredGuard` | No |
| Best for | Auth, rate limit checks, RBAC | Logging, request ID, CORS, body mutation |

Use guards for access control. Use middleware for cross-cutting concerns.