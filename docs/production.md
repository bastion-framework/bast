---
title: Production Checklist
nav_order: 7
---

# Production Checklist
{: .no_toc }

Everything to review before a Bast service takes real traffic.
{: .fs-6 .fw-300 }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Server timeouts

Bast ships safe defaults for the timeouts that can never break your app:

| Setting | Default | Purpose |
|---|---|---|
| `ReadHeaderTimeout` | `10s` | Slowloris protection — bounds how long a client may dribble headers |
| `IdleTimeout` | `120s` | Reaps idle keep-alive connections |
| `ReadTimeout` | unset | **Deliberately** — a default would kill slow uploads |
| `WriteTimeout` | unset | **Deliberately** — a default would kill every SSE stream |

Convention: `0` means "use the default", a negative value disables the timeout entirely.

```go
app := bast.New(bast.Config{
    ReadHeaderTimeout: 5 * time.Second,  // tighten the default
    IdleTimeout:       -1,               // disable (not recommended)
})
```

If your service handles neither streaming nor large uploads, set `ReadTimeout` and
`WriteTimeout` explicitly. Otherwise, protect request duration with `HandlerTimeout`.

## Handler deadlines

`HandlerTimeout` applies a per-request deadline to every non-stream route. Handlers
observe it through `ctx.Context()` — the same context you already pass to your
database and downstream calls.

```go
app := bast.New(bast.Config{
    HandlerTimeout: 10 * time.Second, // global deadline
})

// A route-level Timeout overrides the global one:
bast.GET("/reports", generateReport, bast.WithTimeout(60*time.Second))
```

Stream routes are exempt — they are long-lived by design.

## Panic recovery

Panic recovery is **built in**. A panicking handler produces the standard 500
envelope and a logged stack trace; the connection survives. Stream handlers are
recovered too: a 500 if nothing was written yet, a clean connection close otherwise.
`http.ErrAbortHandler` propagates untouched, per `net/http` convention.

`middleware.Recover` remains available when you want custom recovery behavior —
it runs inside the middleware chain, before the built-in recovery.

## Trusted proxies

`ctx.IP()` honors `X-Forwarded-For` / `X-Real-IP` **only** from proxies you
explicitly trust:

```go
app := bast.New(bast.Config{
    TrustedProxies: []string{"10.0.0.0/8", "172.16.0.0/12"},
})
```

An invalid CIDR **panics at startup**. A silently-dropped entry would change which
headers get trusted — a security control fails loudly or not at all.

## Body size limits

The global default is 4 MB. Override globally with `Config.MaxBodySize` or
per route:

```go
bast.POST("/upload", handleUpload, bast.WithMaxBody(50<<20)) // 50 MB
```

## Graceful shutdown

```go
go func() {
    if err := app.Listen(); err != nil {
        log.Fatal(err)
    }
}()

stop := make(chan os.Signal, 1)
signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
<-stop

// ShutdownTimeout (default 30s) applies when the context has no deadline.
if err := app.Shutdown(context.Background()); err != nil {
    log.Printf("shutdown: %v", err)
}
```

`Shutdown` stops accepting connections, drains in-flight requests, then runs module
`OnShutdown` hooks in reverse registration order, each bounded by `HookTimeout`.

## Kubernetes probes

`/health` (liveness) and `/ready` (readiness) are first-class:

```yaml
livenessProbe:
  httpGet: { path: /health, port: 8080 }
  periodSeconds: 10
readinessProbe:
  httpGet: { path: /ready, port: 8080 }
  periodSeconds: 5
  failureThreshold: 2
```

Wire dependency checks into readiness so the pod is pulled from rotation when a
dependency degrades — see [Health Checks]({{ site.baseurl }}/features/health).

## Startup validation

Bast fails fast on configuration bugs rather than degrading silently:

- `LoadConfig[T]()` errors on missing `required` env vars — and reports **all** of them at once
- Invalid `TrustedProxies` CIDRs panic at startup
- Invalid route patterns panic at registration
- A failing module `OnInit` refuses to start the server

## Checklist

- [ ] `HandlerTimeout` set (or per-route `Timeout` on slow endpoints)
- [ ] `TrustedProxies` configured if behind a load balancer
- [ ] `MaxBodySize` reviewed for upload endpoints
- [ ] SIGTERM → `app.Shutdown` wired in `main.go`
- [ ] Liveness/readiness probes pointed at `/health` and `/ready`
- [ ] Readiness checks registered for critical dependencies
- [ ] A real `Logger` wired (or the default with `NO_COLOR=1` for log collectors)
