---
title: Error Handling
parent: Concepts
nav_order: 6
---

# Error Handling
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

Bast uses a centralized **error boundary** pattern. Handlers return errors via `ctx.Error(err)`. The boundary intercepts every error and maps it to a consistent JSON response. One place — consistent responses across the entire app.

---

## How it works

```
Handler returns ctx.Error(err)
         ↓
   Error Boundary (ErrorHandler)
         ↓
   BastError?       → {"error": {"code": "...", "message": "..."}}
   ValidationError? → {"error": {"code": "VALIDATION_FAILED", "fields": {...}}}
   unknown error    → 500, message hidden from client, logged internally
```

---

## BastError — typed errors

```go
type BastError struct {
    Status  int    // HTTP status code
    Code    string // machine-readable, e.g. "USER_NOT_FOUND"
    Message string // human-readable
}
```

Every `ErrXxx` constructor returns a `*BastError` wrapped as an `error`. The `errors.As` chain is guaranteed — wrapping with `fmt.Errorf` works:

```go
return fmt.Errorf("payment initiation: %w",
    bast.ErrNotFound("WALLET_NOT_FOUND", "wallet not found"))
// ↑ boundary unwraps the chain correctly → 404 response
```

---

## Error codes and constructors

| Constructor | Status | Default Code |
|-------------|--------|--------------|
| `ErrBadRequest(code, msg)` | 400 | `BAD_REQUEST` |
| `ErrInvalidBody(msg)` | 400 | `INVALID_BODY` |
| `ErrUnauthorized(code, msg)` | 401 | `UNAUTHORIZED` |
| `ErrForbidden(code, msg)` | 403 | `FORBIDDEN` |
| `ErrNotFound(code, msg)` | 404 | `NOT_FOUND` |
| `ErrMethodNotAllowed(msg)` | 405 | `METHOD_NOT_ALLOWED` |
| `ErrTimeout(msg)` | 408 | `REQUEST_TIMEOUT` |
| `ErrConflict(code, msg)` | 409 | `CONFLICT` |
| `ErrPayloadTooLarge(msg)` | 413 | `PAYLOAD_TOO_LARGE` |
| `ErrUnprocessable(code, msg)` | 422 | `UNPROCESSABLE_ENTITY` |
| `ErrTooManyRequests(msg)` | 429 | `TOO_MANY_REQUESTS` |
| `ErrInternal(code, msg)` | 500 | `INTERNAL_ERROR` |
| `ErrNotImplemented(msg)` | 501 | `NOT_IMPLEMENTED` |
| `ErrServiceUnavailable(msg)` | 503 | `SERVICE_UNAVAILABLE` |
| `ErrGatewayTimeout(msg)` | 504 | `GATEWAY_TIMEOUT` |

Use the code constants to avoid string typos:

```go
// Instead of:
bast.ErrNotFound("USER_NOT_FOUND", "user not found")

// You can also use your own domain code:
bast.ErrNotFound("ACCOUNT_NOT_FOUND", "no account with id "+id)
```

---

## Error response envelopes

### Standard error

```json
{
  "error": {
    "code": "USER_NOT_FOUND",
    "message": "no user with id 42"
  }
}
```

### Validation error (field-level)

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "request validation failed",
    "fields": {
      "email":    "must be a valid email address",
      "password": "must be at least 8 characters"
    }
  }
}
```

### Internal error (internals never leaked)

```json
{
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "internal server error"
  }
}
```

---

## ValidationError — field-level errors

`ctx.Bind()` returns a `*ValidationError` when struct validation fails. The boundary detects it automatically and produces the field-level 422 response — no manual handling needed.

```go
func (c *UsersController) CreateUser(ctx *bast.Ctx) bast.Response {
    var req CreateUserRequest
    if err := ctx.Bind(&req); err != nil {
        return ctx.Error(err)
        // ↑ if validation failed → 422 with field details
        // ↑ if JSON malformed  → 400 INVALID_BODY
    }
    // ...
}
```

---

## Custom error codes

Define domain-specific codes as constants:

```go
// shared/errors/errors.go
package errors

import "github.com/bastion-framework/bast"

const (
    CodeUserNotFound    = "USER_NOT_FOUND"
    CodeEmailTaken      = "EMAIL_ALREADY_TAKEN"
    CodeInsufficientFunds = "INSUFFICIENT_FUNDS"
    CodePaymentDeclined = "PAYMENT_DECLINED"
)

func UserNotFound(id string) error {
    return bast.ErrNotFound(CodeUserNotFound, "user "+id+" not found")
}

func EmailTaken(email string) error {
    return bast.ErrConflict(CodeEmailTaken, email+" is already registered")
}
```

---

## Custom error handler

Override the default boundary for the whole app:

```go
app := bast.New(bast.Config{
    ErrorHandler: func(ctx *bast.Ctx, err error) bast.Response {
        var bastErr *bast.BastError
        if errors.As(err, &bastErr) {
            // Add request ID to every error response
            reqID, _ := bast.Get[string](ctx, "requestID")
            body := bastErr.JSON()
            return ctx.Raw(bastErr.Status, "application/json", body).
                WithHeader("X-Request-ID", reqID)
        }
        // Unknown errors — log internally, return 500
        slog.Error("unhandled error",
            "err", err,
            "path", ctx.Path(),
            "method", ctx.Method(),
        )
        return ctx.Raw(500, "application/json",
            []byte(`{"error":{"code":"INTERNAL_ERROR","message":"internal server error"}}`))
    },
})
```

---

## Error boundary in the request lifecycle

```
Handler
  └─ returns ctx.Error(err)
       └─ resp.IsError() == true
            └─ Logger.OnError(ctx, err)   ← logged
                 └─ errHandler(ctx, err)  ← boundary
                      └─ writeResponse    ← written to wire
```