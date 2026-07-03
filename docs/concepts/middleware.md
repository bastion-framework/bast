# Middleware

Middleware in Bast is a function that wraps a `HandlerFunc` to form a pipeline. Call `next(ctx)` to pass control downstream; return a `Response` directly to short-circuit.

```go
type MiddlewareFunc func(next HandlerFunc) HandlerFunc
```

---

## Writing middleware

### Pass-through (inspect both request and response)

```go
func Logger(next bast.HandlerFunc) bast.HandlerFunc {
    return func(ctx *bast.Ctx) bast.Response {
        start := time.Now()
        resp := next(ctx)
        slog.Info("request",
            "method", ctx.Method(),
            "path",   ctx.Path(),
            "status", resp.Status(),
            "dur",    time.Since(start),
        )
        return resp
    }
}
```

### Short-circuit (block the request)

```go
func RequireHTTPS(next bast.HandlerFunc) bast.HandlerFunc {
    return func(ctx *bast.Ctx) bast.Response {
        if ctx.Header("X-Forwarded-Proto") != "https" {
            return ctx.Error(bast.ErrForbidden(bast.CodeForbidden, "HTTPS required"))
        }
        return next(ctx)
    }
}
```

### Modify the response (add headers)

```go
func RequestID(next bast.HandlerFunc) bast.HandlerFunc {
    return func(ctx *bast.Ctx) bast.Response {
        id := newRequestID()
        ctx.Set("requestID", id)
        return next(ctx).WithHeader("X-Request-ID", id)
    }
}
```

---

## Registration and execution order

Middleware is registered at three levels and always executes in this guaranteed order:

```
Global Middleware (registration order)
    → Module Middleware (registration order)
        → Route Middleware (registration order)
            → Guards
                → Handler
                    → Response flows back up (reverse order)
```

The pipeline is **built once at registration time** via function composition — not evaluated at request time by iterating a slice. At request time it is a single function call. No slice overhead.

### Global

```go
app.Use(
    middleware.RequestID,
    middleware.Recover,
    middleware.CORS(middleware.CORSConfig{
        AllowedOrigins: []string{"*"},
    }),
)
```

### Module-scoped

```go
bast.Module{
    Prefix:     "/api",
    Middleware: []bast.MiddlewareFunc{
        rateLimitMiddleware,
        auditMiddleware,
    },
}
```

### Route-scoped

```go
bast.POST("/upload", c.Upload,
    bast.WithMiddleware(validateContentType),
)
```

---

## Built-in middleware

All built-in middleware lives in `github.com/bastion-framework/bast/middleware`.

### `middleware.RequestID`

Generates a cryptographically random request ID, stores it in the Ctx store under `"requestID"`, and attaches it as `X-Request-ID` on the response.

```go
app.Use(middleware.RequestID)

// In a handler or downstream middleware:
id, _ := bast.Get[string](ctx, "requestID")
```

### `middleware.Recover`

Catches panics in user handlers, logs them with `slog`, and returns a `500 INTERNAL_ERROR` response. Framework code never panics — this exists only for user handler panics.

```go
app.Use(middleware.Recover)
```

### `middleware.Logger`

Logs each request using `log/slog` with method, path, status, duration, and IP.

```go
app.Use(middleware.Logger)
```

The framework's built-in `[Bast]` logger already logs every request via `Logger.OnRequest`. Add `middleware.Logger` only if you need a separate slog-based log stream.

### `middleware.CORS`

Full CORS implementation with configurable origins, methods, headers, credentials, and preflight handling.

```go
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowedOrigins:   []string{"https://myapp.com", "https://admin.myapp.com"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
}))

// Allow all origins (development only):
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowedOrigins: []string{"*"},
}))
```

`CORSConfig` defaults:
- `AllowedMethods`: `GET, POST, PUT, PATCH, DELETE, OPTIONS`
- `AllowedHeaders`: `Content-Type, Authorization`

---

## Reading the response in middleware

Middleware receives the `Response` returned by the handler. Inspect it to audit errors or attach headers:

```go
func AuditLog(next bast.HandlerFunc) bast.HandlerFunc {
    return func(ctx *bast.Ctx) bast.Response {
        resp := next(ctx)
        if resp.IsError() {
            audit.LogFailure(ctx.Path(), resp.Err())
        }
        return resp // always forward — never swallow
    }
}
```
