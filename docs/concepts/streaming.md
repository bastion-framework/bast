---
title: Streaming
parent: Concepts
nav_order: 7
---

# Streaming — `*bast.StreamCtx`
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

For long-lived connections (SSE, chunked responses, file streaming), Bast provides `*StreamCtx` — a completely different type from `*Ctx` that is **never pooled**.

`*StreamCtx` signals to both the framework and the developer: this connection is long-lived, you own it for its full duration, no recycling happens.

---

## StreamCtx vs Ctx

| | `*Ctx` | `*StreamCtx` |
|--|--------|-------------|
| Lifetime | Single request, pooled | Connection lifetime, GC'd |
| Pool | `sync.Pool` | Never pooled |
| `context.Context` | ❌ Not implemented (safety) | ✅ Embedded directly |
| Returns | `Response` value | Nothing — owns the wire |
| Path params | `ctx.Param("id")` | `sctx.Param("id")` |
| Store access | `ctx.Get("key")` | `sctx.Get("key")` |
| Guards | ✅ Global → module → route | ✅ Same order — runs before handler |
| Use case | Normal request/response | SSE, chunked, streaming |

---

## StreamCtx API

```go
type StreamCtx struct {
    context.Context  // embedded — pass StreamCtx as context.Context directly
    Request *http.Request
    // ...
}

// --- Writing ---

// SetHeader sets a response header. Must be called before first Write or Send.
func (s *StreamCtx) SetHeader(key, value string)

// Send writes a Server-Sent Event to the client.
func (s *StreamCtx) Send(event, data string) error

// Write writes raw bytes to the client.
func (s *StreamCtx) Write(p []byte) (int, error)

// Flush flushes buffered data to the client immediately.
func (s *StreamCtx) Flush()

// Closed returns a channel closed when the client disconnects.
func (s *StreamCtx) Closed() <-chan struct{}

// --- Request ---

// Param returns a URL path parameter by name.
// e.g. STREAM("/:id/events", ...) → sctx.Param("id")
func (s *StreamCtx) Param(key string) string

// --- Store — populated by guards before the handler runs ---

// Get retrieves a value set by a guard.
func (s *StreamCtx) Get(key string) (any, bool)

// Set stores a value in the request-scoped store.
func (s *StreamCtx) Set(key string, val any)

// MustGet retrieves a value and panics if not found.
// Use only when a guard guarantees the value was set.
func (s *StreamCtx) MustGet(key string) any
```

---

## Declaring a streaming route

Use `bast.STREAM` instead of `bast.GET`:

```go
func (c *NotificationsController) Routes() []bast.Route {
    return []bast.Route{
        bast.GET("/:id", c.Get),                    // regular handler
        bast.STREAM("/:id/events", c.HandleSSE),    // streaming handler
    }
}
```

The streaming handler signature:

```go
type StreamHandlerFunc func(ctx *StreamCtx)
```

No return value. The handler owns the wire for its full duration.

---

## Guards on streaming routes

Guards run on streaming routes exactly as they do on regular routes — same registration points, same execution order, same error contract.

```
Global Guards → Module Guards → Route Guards → Stream handler
```

If any guard returns an error, Bast writes the HTTP error response and **never calls the stream handler**. No partial connection, no cleanup needed.

### Attaching guards

Guards attach at the same three levels as regular routes:

```go
// Global — runs on every request including stream routes
app.Guard(guards.AuthGuard)

// Module — runs on every route in the module
bast.Module{
    Prefix:  "/notifications",
    Guards:  []bast.Guard{guards.AuthGuard},
    Controller: notificationsController,
}

// Route — runs on that stream route only
bast.STREAM("/events", c.HandleSSE,
    bast.WithGuards(guards.RequireRole("premium")),
)
```

### Reading the Authorization header in a guard

Guards receive a `*Ctx` and have full access to request headers:

```go
var AuthGuard = bast.GuardFunc(func(ctx *bast.Ctx) error {
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
})
```

---

## Accessing guard values — `sctx.Get` / `sctx.MustGet`

Values set by guards via `ctx.Set` are transferred to `*StreamCtx` automatically. Access them with `sctx.Get` or `sctx.MustGet`:

```go
func (c *NotificationsController) HandleSSE(ctx *bast.StreamCtx) {
    // AuthGuard already ran and set "claims" — safe to MustGet
    claims := ctx.MustGet("claims").(*Claims)

    ctx.SetHeader("Content-Type", "text/event-stream")
    ctx.SetHeader("Cache-Control", "no-cache")
    ctx.SetHeader("X-Accel-Buffering", "no")

    sub := c.service.Subscribe(ctx, claims.UserID)
    defer sub.Close()

    for {
        select {
        case event := <-sub.Events():
            if err := ctx.Send(event.Type, event.Data); err != nil {
                return
            }
            ctx.Flush()
        case <-ctx.Done():
            return
        }
    }
}
```

Use the typed generic helper to avoid type assertions:

```go
// bast.Get[T] works on *Ctx. For *StreamCtx, use sctx.Get and assert yourself:
claims, ok := ctx.Get("claims")
if !ok {
    return // guard should have blocked — defensive check
}
typed := claims.(*Claims)
```

---

## Path parameters — `sctx.Param`

Path parameters extracted by the router are available via `sctx.Param`:

```go
// Route: STREAM("/:userID/events", c.HandleSSE)
func (c *NotificationsController) HandleSSE(ctx *bast.StreamCtx) {
    userID := ctx.Param("userID")

    sub := c.service.Subscribe(ctx, userID)
    defer sub.Close()

    for {
        select {
        case event := <-sub.Events():
            ctx.Send(event.Type, event.Data)
            ctx.Flush()
        case <-ctx.Done():
            return
        }
    }
}
```

Missing params return an empty string — the same zero-value behaviour as `ctx.Param` on regular routes.

---

## Complete SSE example with guards and path params

```go
// Module wiring
func NewModule(db *pgxpool.Pool) bast.Module {
    svc        := newService(db)
    controller := newController(svc)

    return bast.Module{
        Prefix:     "/notifications",
        Guards:     []bast.Guard{guards.AuthGuard},   // applies to stream routes too
        Controller: controller,
    }
}

// Controller
func (c *Controller) Routes() []bast.Route {
    return []bast.Route{
        bast.STREAM("/:roomID/events", c.Stream),
    }
}

// Handler — guards have already run when this is called
func (c *Controller) Stream(ctx *bast.StreamCtx) {
    roomID := ctx.Param("roomID")
    claims := ctx.MustGet("claims").(*Claims)

    ctx.SetHeader("Content-Type", "text/event-stream")
    ctx.SetHeader("Cache-Control", "no-cache")
    ctx.SetHeader("X-Accel-Buffering", "no")

    sub := c.service.Subscribe(ctx, roomID, claims.UserID)
    defer sub.Close()

    for {
        select {
        case msg := <-sub.Messages():
            if err := ctx.Send("message", msg); err != nil {
                return
            }
            ctx.Flush()
        case <-ctx.Done():
            return
        }
    }
}
```

---

## Chunked file streaming

```go
func (c *FilesController) Stream(ctx *bast.StreamCtx) {
    ctx.SetHeader("Content-Type", "application/octet-stream")
    ctx.SetHeader("Content-Disposition", `attachment; filename="export.csv"`)

    rows, err := c.service.ExportRows(ctx)
    if err != nil {
        ctx.Write([]byte("error: " + err.Error()))
        return
    }

    w := csv.NewWriter(ctx) // ctx implements io.Writer
    for _, row := range rows {
        w.Write(row)
    }
    w.Flush()
}
```

---

## Why two separate types?

The design makes misuse impossible at compile time:

```go
// Can't accidentally pool a streaming connection:
// StreamCtx is never in the pool — it's a different type entirely.

// Can't accidentally ignore context cancellation:
// StreamCtx.Done() is always available via the embedded context.
```

If you try to use a `StreamHandlerFunc` where a `HandlerFunc` is expected (or vice versa), the compiler rejects it immediately.

---

## Testing stream handlers

Use `basttest.NewStreamCtx` to build a `*StreamCtx` outside the pool for unit tests:

```go
import "github.com/bastion-framework/bast/basttest"

func TestStream_ReadsParam(t *testing.T) {
    ctx := basttest.NewStreamCtx(
        basttest.WithStreamParam("roomID", "room-42"),
        basttest.WithStreamStore("claims", &Claims{UserID: "u-1", Role: "user"}),
    )

    roomID := ctx.Param("roomID")
    if roomID != "room-42" {
        t.Errorf("Param = %q, want room-42", roomID)
    }

    claims := ctx.MustGet("claims").(*Claims)
    if claims.UserID != "u-1" {
        t.Errorf("UserID = %q, want u-1", claims.UserID)
    }
}
```

Available options:

| Option | Description |
|--------|-------------|
| `WithStreamParam(key, value)` | Set a path parameter |
| `WithStreamStore(key, val)` | Pre-populate store (simulate guard output) |
| `WithStreamHeader(key, value)` | Set a request header |
| `WithStreamMethod(method)` | Set the HTTP method |
| `WithStreamPath(path)` | Set the URL path |
