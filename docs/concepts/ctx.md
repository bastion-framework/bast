# Context — `*bast.Ctx`

`*bast.Ctx` is the heart of Bast. Every handler receives one. It provides typed access to the request, route params, headers, body, and response utilities.

---

## Critical design decision

`*Ctx` deliberately does **not** implement `context.Context`. This is structural safety — the compiler prevents you from accidentally passing a pooled `*Ctx` to a goroutine or service:

```go
// These are compile errors — intentional:
var _ context.Context = (*bast.Ctx)(nil)  // ❌ compile error
go func(ctx context.Context) {}(ctx)      // ❌ compile error
service.GetUser(ctx, id)                  // ❌ if service expects context.Context

// You are forced to do the correct thing:
stdctx := ctx.Context()                   // ✅ detached, safe, pool-independent
go func(ctx context.Context) {}(stdctx)   // ✅
service.GetUser(stdctx, id)               // ✅
```

This makes pool-related data races **impossible by construction**, not just by documentation.

---

## Ctx is pooled

`*Ctx` instances are recycled via `sync.Pool`. The pool acquire+release cycle costs **23 ns / 0 allocs**. When a request completes, all fields are zeroed and the `*Ctx` returns to the pool.

**Never store a `*Ctx` beyond the handler's return.** Always extract what you need via `ctx.Context()` for anything that outlives the handler.

---

## Context propagation

### `ctx.Context() context.Context`

Returns a detached `context.Context` safe to pass anywhere. It carries all values, deadlines, and cancellation from the request. This is the **only** safe way to get a context from `*Ctx` for use outside the handler.

```go
func (c *UsersController) GetUser(ctx *bast.Ctx) bast.Response {
    // Pass to service, repo, DB calls — always use ctx.Context()
    user, err := c.service.GetUser(ctx.Context(), ctx.Param("id"))
    if err != nil {
        return ctx.Error(err)
    }
    return ctx.OK(user)
}
```

### `ctx.WithValue(key, val any) *Ctx`

Returns a shallow copy of `Ctx` with a value injected into the context. Used by middleware to propagate typed values downstream.

```go
func InjectTenant(next bast.HandlerFunc) bast.HandlerFunc {
    return func(ctx *bast.Ctx) bast.Response {
        tenantID := ctx.Header("X-Tenant-ID")
        ctx = ctx.WithValue("tenantID", tenantID)
        return next(ctx)
    }
}
```

---

## Request access

### `ctx.Param(key string) string`

Returns a URL path parameter by name. Lookup is a linear O(N) scan over a `[8]Param` array embedded in the Ctx struct — **0 allocations**, faster than a map for the typical N≤8.

```go
// Route: /users/:id
ctx.Param("id") // → "42"

// Route: /orgs/:org/repos/:repo
ctx.Param("org")  // → "bastion-framework"
ctx.Param("repo") // → "bast"
```

### `ctx.Query(key string) string`

Returns a URL query parameter by name.

```go
// URL: /users?page=2&limit=20
ctx.Query("page")  // → "2"
ctx.Query("limit") // → "20"
ctx.Query("missing") // → ""
```

### `ctx.QueryDefault(key, fallback string) string`

Returns the query param or `fallback` if the key is absent.

```go
page := ctx.QueryDefault("page", "1")
limit := ctx.QueryDefault("limit", "20")
```

### `ctx.Header(key string) string`

Returns a request header value by name. Case-insensitive.

```go
token := ctx.Header("Authorization") // → "Bearer eyJ..."
ct    := ctx.Header("Content-Type")  // → "application/json"
```

### `ctx.IP() string`

Returns the real client IP. Respects `X-Forwarded-For` and `X-Real-IP` **only** when the request originates from a configured trusted proxy. Prevents IP spoofing by external clients.

```go
app := bast.New(bast.Config{
    TrustedProxies: []string{
        "10.0.0.0/8",
        "172.16.0.0/12",
    },
})

// In handler:
ip := ctx.IP()
```

### `ctx.Method() string` / `ctx.Path() string`

```go
ctx.Method() // → "GET", "POST", etc.
ctx.Path()   // → "/users/42"
```

---

## Body parsing

### `ctx.Bind(v any) error`

Decodes and validates the request body into `v`. Returns a `400 INVALID_BODY` error on malformed JSON and a `ValidationError` (422) on failed struct validation. The body is buffered on first read — safe to call multiple times.

```go
func (c *UsersController) CreateUser(ctx *bast.Ctx) bast.Response {
    var req CreateUserRequest
    if err := ctx.Bind(&req); err != nil {
        return ctx.Error(err) // automatically → 400 or 422
    }
    // req is decoded and validated
}
```

### `ctx.BindJSON(v any) error`

Like `Bind` but skips validation. Use when you handle validation manually.

### `ctx.RawBody() ([]byte, error)`

Returns the raw body bytes. Safe to call multiple times — body is buffered after first read.

```go
raw, err := ctx.RawBody()
```

### File uploads

```go
// Single file
fh, err := ctx.File("avatar")

// Multiple files from one field
fhs, err := ctx.Files("attachments")

// Form values alongside files
name := ctx.FormValue("name")
```

---

## Middleware store

The store is a request-scoped `map[string]any` for passing values between middleware and handlers. Concurrency-safe: `*Ctx` is single-goroutine by design.

### `ctx.Set(key string, val any)` / `ctx.Get(key string) (any, bool)`

```go
// In middleware:
ctx.Set("requestID", uuid.New().String())

// In handler:
id, ok := ctx.Get("requestID")
```

### `bast.Get[T](ctx, key)` — typed getter (no assertion needed)

```go
// Before (noisy):
val, _ := ctx.Get("claims")
claims := val.(*Claims) // can panic

// After (clean):
claims, ok := bast.Get[*Claims](ctx, "claims")
```

### `bast.MustGet[T](ctx, key)` — panics if missing

Use only after a guard has guaranteed the value is present:

```go
// Guard sets claims. Handler retrieves them safely:
claims := bast.MustGet[*Claims](ctx, "claims")
```

---

## Response builders

All builders return a `Response` value — they **never** write to the wire. The framework writes the response after the handler returns, allowing middleware to inspect or modify it.

| Method | Status | Body |
|--------|--------|------|
| `ctx.OK(data)` | 200 | `{"data": ..., "meta": null}` |
| `ctx.Created(data)` | 201 | `{"data": ..., "meta": null}` |
| `ctx.NoContent()` | 204 | empty |
| `ctx.Paginated(data, meta)` | 200 | `{"data": [...], "meta": {...}}` |
| `ctx.Redirect(url, code)` | 3xx | redirect |
| `ctx.Error(err)` | via boundary | `{"error": {...}}` |
| `ctx.Raw(status, ct, body)` | any | raw bytes |

### Pagination

```go
func (c *PostsController) List(ctx *bast.Ctx) bast.Response {
    posts, total, err := c.service.List(ctx.Context(), page, limit)
    if err != nil {
        return ctx.Error(err)
    }
    return ctx.Paginated(posts, bast.PaginationMeta{
        Page:    page,
        PerPage: limit,
        Total:   total,
        Pages:   (total + limit - 1) / limit,
    })
}
```

Response:

```json
{
  "data": [...],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 340,
    "pages": 17
  }
}
```

### Cookies

```go
func (c *AuthController) Login(ctx *bast.Ctx) bast.Response {
    // ... auth logic ...
    return ctx.OK(user).WithCookie(&http.Cookie{
        Name:     "session",
        Value:    sessionToken,
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
        MaxAge:   86400,
    })
}
```
