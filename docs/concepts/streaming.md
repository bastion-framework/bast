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
| Use case | Normal request/response | SSE, chunked, streaming |

---

## StreamCtx API

```go
type StreamCtx struct {
    context.Context              // embedded — pass StreamCtx as context.Context directly
    Request  *http.Request
    // ...
}

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
```

---

## Declaring a streaming route

Use `bast.STREAM` instead of `bast.GET`:

```go
func (c *NotificationsController) Routes() []bast.Route {
    return []bast.Route{
        bast.GET("/:id", c.Get),               // regular handler
        bast.STREAM("/events", c.HandleSSE),   // streaming handler
    }
}
```

The streaming handler signature:

```go
type StreamHandlerFunc func(ctx *StreamCtx)
```

No return value. The handler owns the wire for its full duration.

---

## Server-Sent Events (SSE)

```go
func (c *NotificationsController) HandleSSE(ctx *bast.StreamCtx) {
    ctx.SetHeader("Content-Type", "text/event-stream")
    ctx.SetHeader("Cache-Control", "no-cache")
    ctx.SetHeader("X-Accel-Buffering", "no") // disable nginx buffering

    userID := ctx.Request.URL.Query().Get("userID")
    sub := c.service.Subscribe(ctx, userID)
    defer sub.Close()

    for {
        select {
        case event := <-sub.Events():
            if err := ctx.Send(event.Type, event.Data); err != nil {
                return // client disconnected
            }
            ctx.Flush()

        case <-ctx.Done(): // embedded context — client gone or server shutting down
            return
        }
    }
}
```

Because `*StreamCtx` embeds `context.Context`, you can pass it directly to services, DB calls, and goroutines without calling `.Context()` first:

```go
sub := c.service.Subscribe(ctx, userID) // ✅ ctx IS a context.Context
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
var _ sync.Pool // StreamCtx is never in the pool — it's a different type

// Can't accidentally ignore context cancellation:
// StreamCtx.Done() is always available via the embedded context
```

If you try to use a `StreamHandlerFunc` where a `HandlerFunc` is expected (or vice versa), the compiler rejects it immediately.