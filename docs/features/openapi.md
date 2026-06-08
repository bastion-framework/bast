---
title: OpenAPI / Swagger
parent: Features
nav_order: 2
---

# OpenAPI / Swagger
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

Bast generates an OpenAPI 3.0.3 spec directly from your code — no comment annotations, no external codegen tools. The spec is built once at startup from route metadata and served statically. Zero reflection at request time.

---

## Configuration

```go
app := bast.New(bast.Config{
    Docs: &bast.DocsConfig{
        Enabled:     true,
        Path:        "/docs",         // Swagger UI
        JSONPath:    "/openapi.json", // raw OpenAPI 3.0.3 spec
        Title:       "My API",
        Version:     "1.0.0",
        Description: "Production API built with Bast",
    },
})
```

Visit `/docs` for the Swagger UI and `/openapi.json` for the raw spec.

---

## Route documentation

Attach a `bast.Doc{}` to any route:

```go
bast.GET("/:id", c.GetUser, bast.WithDoc(bast.Doc{
    Summary:     "Get a user by ID",
    Description: "Returns a single user. Requires authentication.",
    Tags:        []string{"Users"},
    Params: []bast.Param{
        bast.PathParam("id", "User UUID"),
        bast.QueryParam("include", "Comma-separated relations to include"),
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

---

## `bast.Body[T]()` — schema generation

`bast.Body[T]()` captures the type via reflection **once at startup**. Bast reads `json` struct tags for field names and `validate` tags for constraints (required, format, min, max):

```go
type CreateUserRequest struct {
    Name     string `json:"name"     validate:"required,min=2,max=100"`
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    Role     string `json:"role"     validate:"required,oneof=admin user"`
}
```

This generates an OpenAPI schema with required fields, string formats, and length constraints.

---

## Module tag groups

Set `Doc` on a module to create a named tag group in the Swagger UI:

```go
bast.Module{
    Prefix: "/users",
    Doc: bast.ModuleDoc{
        Name:        "Users",
        Description: "User management — create, read, update, delete users",
    },
    Controller: controller,
}
```

Routes with `Tags: []string{"Users"}` in their `Doc` will appear under this group.

---

## Security schemes — `SecuredGuard`

Guards that implement `SecuredGuard` automatically contribute to `securitySchemes`:

```go
type jwtGuard struct{}

func (g *jwtGuard) SecurityScheme() bast.SecurityScheme {
    return bast.SecurityScheme{
        Type:         "http",
        Scheme:       "bearer",
        BearerFormat: "JWT",
        Description:  "JWT access token from POST /auth/login",
    }
}
```

When this guard is registered on a module or route, Bast marks those routes as secured in the spec and adds the scheme to `securitySchemes`. The Swagger UI shows the lock icon and the "Authorize" button automatically.

---

## Deprecated routes

```go
bast.GET("/:id", c.GetUser, bast.WithDoc(bast.Doc{
    Summary:       "Get a user by ID",
    Deprecated:    true,
    DeprecatedMsg: "Use GET /v2/users/:id instead",
}))
```

The route is marked deprecated in Swagger UI with a strikethrough.

---

## What Bast infers without `Doc{}`

Even with zero `Doc{}` on a route, Bast still generates:

- HTTP method and path
- Authentication requirement (if a `SecuredGuard` is attached)
- Path parameters (auto-extracted from the pattern)

`Doc{}` is additive — human-readable descriptions on top of what Bast already knows.

---

## Disable in production

```go
Docs: &bast.DocsConfig{
    Enabled: os.Getenv("APP_ENV") != "production",
    // ...
},
```