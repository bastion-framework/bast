---
title: Configuration
parent: Features
nav_order: 4
---

# Configuration — `LoadConfig[T]`
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

`bast.LoadConfig[T]()` loads typed, validated configuration from environment variables at startup. Missing required variables are **all** reported in a single error so you fix them in one deploy cycle, not one at a time.

Reflection runs once at startup — never at request time.

---

## Basic usage

```go
type AppConfig struct {
    Port        int    `env:"PORT"         default:"8080"`
    DatabaseURL string `env:"DATABASE_URL"  required:"true"`
    JWTSecret   string `env:"JWT_SECRET"    required:"true" secret:"true"`
    Env         string `env:"APP_ENV"       default:"development"`
}

cfg, err := bast.LoadConfig[AppConfig]()
if err != nil {
    // e.g. "bast: missing required config: DATABASE_URL, JWT_SECRET"
    os.Exit(1)
}

fmt.Println(cfg.Port)        // 8080
fmt.Println(cfg.DatabaseURL) // postgres://...
```

---

## Struct tags

| Tag | Description |
|-----|-------------|
| `env:"VAR_NAME"` | Environment variable name (required) |
| `default:"value"` | Value used when the env var is absent |
| `required:"true"` | App refuses to start if absent and no default |
| `secret:"true"` | Value is masked in boot logs |

Unexported fields are skipped — an `env` tag on an unexported field is a
configuration bug, but Bast degrades gracefully instead of panicking at startup.

---

## Supported field types

| Go type | Example env value |
|---------|------------------|
| `string` | `"hello"` |
| `int` | `"8080"` |
| `int64` | `"1073741824"` |
| `float64` | `"3.14"` |
| `bool` | `"true"`, `"false"`, `"1"`, `"0"` |
| `time.Duration` | `"30s"`, `"5m"`, `"1h30m"` |
| `[]string` | `"a,b,c"` → `[]string{"a","b","c"}` |

---

## Nested structs

Group related config with nested structs. The parent field's `env` tag becomes a prefix:

```go
type DatabaseConfig struct {
    URL      string `env:"URL"       required:"true"`
    MaxConns int    `env:"MAX_CONNS" default:"10"`
    SSLMode  string `env:"SSL_MODE"  default:"require"`
}

type RedisConfig struct {
    URL      string `env:"URL"      required:"true"`
    Password string `env:"PASSWORD" secret:"true"`
}

type AppConfig struct {
    Port     int            `env:"PORT"    default:"8080"`
    Database DatabaseConfig `env:"DB"`    // env keys: DB_URL, DB_MAX_CONNS, DB_SSL_MODE
    Redis    RedisConfig    `env:"REDIS"` // env keys: REDIS_URL, REDIS_PASSWORD
    JWTSecret string        `env:"JWT_SECRET" required:"true" secret:"true"`
}

cfg, err := bast.LoadConfig[AppConfig]()
```

Required environment variables: `DB_URL`, `REDIS_URL`, `JWT_SECRET`.

---

## Error reporting

If multiple required variables are missing, all of them are reported in a single error:

```
bast: missing required config: DATABASE_URL, JWT_SECRET, REDIS_URL
```

One error, three variables, one fix cycle.

---

## Full example

```go
package main

import (
    "os"
    "time"

    "github.com/bastion-framework/bast"
)

type AppConfig struct {
    Port     int    `env:"PORT"      default:"8080"`
    Env      string `env:"APP_ENV"   default:"development"`

    Database struct {
        URL      string `env:"URL"       required:"true"`
        MaxConns int    `env:"MAX_CONNS" default:"25"`
    } `env:"DB"`

    Auth struct {
        JWTSecret      string        `env:"JWT_SECRET"       required:"true" secret:"true"`
        TokenExpiry    time.Duration `env:"TOKEN_EXPIRY"     default:"24h"`
        RefreshExpiry  time.Duration `env:"REFRESH_EXPIRY"   default:"168h"`
    } `env:"AUTH"`

    AllowedOrigins []string `env:"ALLOWED_ORIGINS" default:"http://localhost:3000"`
}

func main() {
    cfg, err := bast.LoadConfig[AppConfig]()
    if err != nil {
        os.Exit(1)
    }

    app := bast.New(bast.Config{
        Port:           cfg.Port,
        HandlerTimeout: 10 * time.Second,
    })

    // use cfg.Database.URL to connect, cfg.Auth.JWTSecret for JWT, etc.
    app.Listen()
}
```

---

## Server settings — `bast.Config`

`bast.Config` (passed to `bast.New`) controls the HTTP server itself. Timeout
convention: `0` uses the safe default where one exists, a negative value disables.

| Field | Default | Notes |
|-------|---------|-------|
| `ReadHeaderTimeout` | `10s` | Slowloris protection |
| `IdleTimeout` | `120s` | Keep-alive reaping |
| `ReadTimeout` / `WriteTimeout` | unset | No default — would break streams and uploads |
| `HandlerTimeout` | off | Global per-request deadline via `ctx.Context()` |
| `ShutdownTimeout` | `30s` | Applied when `Shutdown`'s context has no deadline |
| `HookTimeout` | `10s` | Bounds each `OnInit` / `OnShutdown` hook |
| `MaxBodySize` | `4 MB` | Global body limit; override per route with `WithMaxBody` |
| `TrustedProxies` | none | CIDRs allowed to set `X-Forwarded-For`; invalid entries panic |

See the [Production Checklist]({{ site.baseurl }}/production) for how these fit together.