# Health Checks

Every production service needs `/health` and `/ready`. In Bast, health checks are first-class — not a route hack.

---

## Configuration

```go
app := bast.New(bast.Config{
    Health: &bast.HealthConfig{
        LivePath:  "/health",  // liveness  — is the process alive?
        ReadyPath: "/ready",   // readiness — is it ready to serve traffic?
        Checks: []bast.HealthCheck{
            bast.CustomCheck("postgres", func(ctx context.Context) error {
                return pool.Ping(ctx)
            }),
            bast.CustomCheck("redis", func(ctx context.Context) error {
                return redisClient.Ping(ctx).Err()
            }),
            bast.CustomCheck("payments-api", func(ctx context.Context) error {
                return paymentsClient.Ping(ctx)
            }),
        },
    },
})
```

---

## Liveness — `/health`

The liveness probe tells Kubernetes whether the process is alive. It **never** checks dependencies — only that the process is running. It always returns `200`.

```
GET /health
→ 200 OK
{ "status": "alive" }
```

Use for **livenessProbe** in Kubernetes. If this fails, the container is restarted.

---

## Readiness — `/ready`

The readiness probe checks all registered dependencies. Returns `200` if all checks pass, `503` if any check is degraded.

```
GET /ready
→ 200 OK (all healthy)
{
  "status": "healthy",
  "checks": {
    "postgres":     { "status": "healthy",  "latency": "1.2ms" },
    "redis":        { "status": "healthy",  "latency": "0.4ms" },
    "payments-api": { "status": "healthy",  "latency": "3.1ms" }
  }
}
```

```
GET /ready
→ 503 Service Unavailable (any degraded)
{
  "status": "degraded",
  "checks": {
    "postgres":     { "status": "healthy",  "latency": "1.2ms" },
    "payments-api": { "status": "degraded", "latency": "5.0s", "error": "dial timeout" }
  }
}
```

Use for **readinessProbe** in Kubernetes. If this fails, the pod is removed from the load balancer until it recovers.

---

## `bast.CustomCheck`

```go
bast.CustomCheck(name string, fn func(ctx context.Context) error) HealthCheck
```

The check receives the request context — if the readiness endpoint itself has a timeout, checks will be cancelled accordingly. Return `nil` for healthy, any non-nil error for degraded.

```go
bast.CustomCheck("stripe", func(ctx context.Context) error {
    _, err := stripeClient.Balance.Get(&stripe.BalanceParams{})
    return err
})
```

---

## Kubernetes config

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  failureThreshold: 3
```
