---
title: Testing
parent: Features
nav_order: 1
---

# Testing with `basttest`
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

`basttest` ships with the framework — no separate install or mock library needed. It provides two testing patterns: unit tests via `NewCtx` and integration tests via `NewApp`.

```go
import "github.com/bastion-framework/bast/basttest"
```

---

## Unit testing — `basttest.NewCtx`

Builds a `*bast.Ctx` outside the pool, safe for direct use in tests. No HTTP server. Tests a handler as a pure function.

```go
func NewCtx(opts ...CtxOption) *bast.Ctx
```

### Available options

| Option | Description |
|--------|-------------|
| `WithParam(key, value)` | Set a URL path parameter |
| `WithQuery(key, value)` | Set a query string parameter |
| `WithHeader(key, value)` | Set a request header |
| `WithMethod(method)` | Set the HTTP method |
| `WithPath(path)` | Set the URL path |
| `WithBody(v)` | Marshal `v` to JSON, set Content-Type |
| `WithRawBody(b)` | Set raw body bytes |
| `WithStore(key, val)` | Pre-populate the Ctx store (simulate guard output) |
| `WithIP(ip)` | Set the client IP |

### Example — handler unit test

```go
func TestGetUser(t *testing.T) {
    mockService := &MockUsersService{
        GetUserFn: func(ctx context.Context, id string) (*User, error) {
            return &User{ID: id, Name: "Kasim"}, nil
        },
    }

    controller := newController(mockService)

    ctx := basttest.NewCtx(
        basttest.WithParam("id", "42"),
        basttest.WithHeader("Authorization", "Bearer test-token"),
        // Simulate AuthGuard having already set claims:
        basttest.WithStore("claims", &Claims{UserID: "42", Role: "admin"}),
    )

    resp := controller.GetUser(ctx)

    if resp.Status() != 200 {
        t.Errorf("status = %d, want 200", resp.Status())
    }
    if !bytes.Contains(resp.Body(), []byte("Kasim")) {
        t.Errorf("body missing name: %s", resp.Body())
    }
}
```

### Testing error paths

```go
func TestGetUser_NotFound(t *testing.T) {
    mockService := &MockUsersService{
        GetUserFn: func(ctx context.Context, id string) (*User, error) {
            return nil, bast.ErrNotFound("USER_NOT_FOUND", "not found")
        },
    }

    controller := newController(mockService)
    ctx := basttest.NewCtx(basttest.WithParam("id", "99"))

    resp := controller.GetUser(ctx)

    if !resp.IsError() {
        t.Error("expected error response")
    }
}
```

### Testing guards directly

```go
func TestAuthGuard_MissingToken(t *testing.T) {
    ctx := basttest.NewCtx() // no Authorization header

    err := guards.AuthGuard.Check(ctx)
    if err == nil {
        t.Fatal("expected error for missing token")
    }

    var bastErr *bast.BastError
    if !errors.As(err, &bastErr) || bastErr.Status != 401 {
        t.Errorf("expected 401, got %v", err)
    }
}
```

---

## Integration testing — `basttest.NewApp`

Wraps `httptest` with Bast's full module system. Tests the complete request lifecycle including routing, guards, middleware, and the error boundary.

```go
func NewApp(modules ...bast.Module) *TestApp
```

### TestApp methods

```go
// Do sends a request through the full app stack
func (a *TestApp) Do(method, path string, opts ...RequestOption) *TestResponse

// Close shuts down the test server — call in t.Cleanup or defer
func (a *TestApp) Close()
```

### RequestOption

| Option | Description |
|--------|-------------|
| `WithJSONBody(v)` | Marshal `v` as the request body |
| `WithRequestHeader(key, val)` | Set a request header |

### TestResponse

```go
type TestResponse struct {
    Code    int
    Body    []byte
    Headers http.Header
}

func (r *TestResponse) JSON(v any) error          // unmarshal body
func (r *TestResponse) Assert(t) *Assertions      // fluent assertions
```

### Assertions

```go
resp.Assert(t).
    StatusIs(201).
    BodyContains("id").
    HeaderIs("Content-Type", "application/json")
```

### Example — full CRUD integration test

```go
func TestTodosIntegration(t *testing.T) {
    app := basttest.NewApp(todos.NewModule())
    t.Cleanup(app.Close)

    // Create a todo
    resp := app.Do("POST", "/todos/",
        basttest.WithJSONBody(map[string]string{"title": "Buy milk"}),
    )
    resp.Assert(t).StatusIs(201).BodyContains("Buy milk")

    // Extract the ID
    var created struct {
        Data struct{ ID string `json:"id"` } `json:"data"`
    }
    resp.JSON(&created)
    id := created.Data.ID

    // Get it back
    app.Do("GET", "/todos/"+id).
        Assert(t).StatusIs(200).BodyContains("Buy milk")

    // Mark done
    app.Do("PATCH", "/todos/"+id,
        basttest.WithJSONBody(map[string]bool{"done": true}),
    ).Assert(t).StatusIs(200)

    // Delete it
    app.Do("DELETE", "/todos/"+id).
        Assert(t).StatusIs(204)

    // Gone
    app.Do("GET", "/todos/"+id).
        Assert(t).StatusIs(404)
}
```

### Testing with guards

```go
func TestProtectedRoute(t *testing.T) {
    mod := bast.Module{
        Prefix:     "/items",
        Controller: &itemController{},
        Guards:     []bast.Guard{guards.AuthGuard},
    }
    app := basttest.NewApp(mod)
    t.Cleanup(app.Close)

    // Without auth → 401
    app.Do("GET", "/items/").Assert(t).StatusIs(401)

    // With auth → 200
    app.Do("GET", "/items/",
        basttest.WithRequestHeader("Authorization", "Bearer "+validToken),
    ).Assert(t).StatusIs(200)
}
```

### Testing 404 and 405

```go
func TestNotFound(t *testing.T) {
    app := basttest.NewApp(todos.NewModule())
    t.Cleanup(app.Close)

    app.Do("GET", "/missing").Assert(t).StatusIs(404)
    app.Do("DELETE", "/todos/").Assert(t).StatusIs(405) // DELETE not registered on /
}
```

---

## Concurrency safety note

`*Ctx` is single-goroutine by design. Middleware and handlers run sequentially in the same goroutine — `ctx.Set`/`ctx.Get` does not need synchronization. This is guaranteed by Bast's single-goroutine request model.