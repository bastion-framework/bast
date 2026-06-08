---
title: Controllers
parent: Concepts
nav_order: 3
---

# Controllers
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

A controller declares a module's route mappings. It is the only piece of a module that the framework touches directly — through the `Controller` interface.

---

## The Controller interface

```go
type Controller interface {
    Routes() []Route
}
```

That's it. One method. Every controller must implement it.

---

## Declaring routes

```go
type UsersController struct {
    service *UsersService
}

func newController(s *UsersService) *UsersController {
    return &UsersController{service: s}
}

func (c *UsersController) Routes() []bast.Route {
    return []bast.Route{
        bast.GET("/", c.List),
        bast.GET("/:id", c.GetUser),
        bast.POST("/", c.CreateUser),
        bast.PATCH("/:id", c.UpdateUser),
        bast.DELETE("/:id", c.DeleteUser),
    }
}
```

Available route constructors:

```go
bast.GET(pattern, handler, ...opts)
bast.POST(pattern, handler, ...opts)
bast.PUT(pattern, handler, ...opts)
bast.PATCH(pattern, handler, ...opts)
bast.DELETE(pattern, handler, ...opts)
bast.STREAM(pattern, streamHandler, ...opts) // SSE / chunked
```

Patterns are relative to the module's `Prefix`. If the module has `Prefix: "/users"` and a route has `Pattern: "/:id"`, the full path is `/users/:id`.

---

## Handler signature

Every handler has this exact signature — no exceptions:

```go
type HandlerFunc func(ctx *Ctx) Response
```

No variadic args. No `interface{}`. No side effects. No writing to `http.ResponseWriter` directly. The handler returns a `Response` value and the framework writes it to the wire after the handler returns.

---

## Writing handlers

```go
func (c *UsersController) GetUser(ctx *bast.Ctx) bast.Response {
    id := ctx.Param("id")

    user, err := c.service.GetUser(ctx.Context(), id)
    if err != nil {
        return ctx.Error(err) // flows to the error boundary
    }

    return ctx.OK(user)
}

func (c *UsersController) CreateUser(ctx *bast.Ctx) bast.Response {
    var req CreateUserRequest
    if err := ctx.Bind(&req); err != nil {
        return ctx.Error(err) // 400 or 422 automatically
    }

    user, err := c.service.CreateUser(ctx.Context(), req)
    if err != nil {
        return ctx.Error(err)
    }

    return ctx.Created(user)
}
```

---

## Route options

Options are applied with functional option arguments:

```go
bast.GET("/:id", c.GetUser,
    bast.WithGuards(guards.RequireRole("admin")),
    bast.WithTimeout(5 * time.Second),
    bast.WithDoc(bast.Doc{
        Summary: "Get a user by ID",
        Tags:    []string{"Users"},
    }),
)
```

| Option | Purpose |
|--------|---------|
| `WithMiddleware(mw...)` | Attach middleware to this route only |
| `WithGuards(guards...)` | Attach guards to this route only |
| `WithTimeout(d)` | Per-route handler timeout |
| `WithMaxBody(n)` | Per-route body size limit |
| `WithDoc(doc)` | OpenAPI documentation |

---

## Route-level middleware

```go
func (c *UsersController) Routes() []bast.Route {
    return []bast.Route{
        bast.GET("/export", c.Export,
            bast.WithTimeout(60*time.Second),  // exports can take longer
            bast.WithMaxBody(0),               // no body for GETs
        ),
        bast.POST("/import", c.Import,
            bast.WithMaxBody(32*1024*1024),    // 32 MB for file uploads
            bast.WithMiddleware(auditMiddleware),
        ),
    }
}
```

---

## Route-level guards

```go
func (c *UsersController) Routes() []bast.Route {
    return []bast.Route{
        bast.GET("/", c.List),       // any authenticated user
        bast.POST("/", c.Create,
            bast.WithGuards(guards.RequireRole("admin")), // admin only
        ),
        bast.DELETE("/:id", c.Delete,
            bast.WithGuards(guards.RequireRole("admin")),
        ),
    }
}
```

---

## Documenting routes (OpenAPI)

Add a `bast.Doc{}` to any route to enrich the generated OpenAPI spec:

```go
bast.GET("/:id", c.GetUser, bast.WithDoc(bast.Doc{
    Summary:     "Get a user by ID",
    Description: "Returns a single user. Requires authentication.",
    Tags:        []string{"Users"},
    Params: []bast.Param{
        bast.PathParam("id", "User UUID"),
    },
    Returns: bast.Returns{
        200: bast.Body[UserResponse](),
        401: bast.Body[bast.BastError](),
        404: bast.Body[bast.BastError](),
    },
})),

bast.POST("/", c.CreateUser, bast.WithDoc(bast.Doc{
    Summary: "Create a new user",
    Body:    bast.Body[CreateUserRequest](),
    Returns: bast.Returns{
        201: bast.Body[UserResponse](),
        400: bast.Body[bast.BastError](),
        409: bast.Body[bast.BastError](),
    },
})),
```

Bast uses reflection on the types passed to `bast.Body[T]()` **once at startup** to build schemas — zero reflection at request time.