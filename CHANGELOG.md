# Changelog

All notable changes to Bast are documented here.

This project follows [Semantic Versioning](https://semver.org) and the format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Fixed

- *(stream)* Strean route guard + path param gaps ([`686ba5e`](https://github.com/bastion-framework/bast/commit/686ba5ecbf6118148af2a0ecf4712894a14dffc6))
- Address some risk issues ([`1d6629a`](https://github.com/bastion-framework/bast/commit/1d6629a739526bc48ff9b879a56c10fe006be3a2))

---

## [0.1.0] — 2026-06-08

Initial release of the Bast framework.

### Core

- `bast.Ctx` — pooled request context via `sync.Pool`; `*Ctx` deliberately does not implement `context.Context` (structural pool safety)
- `bast.Response` — immutable value type; all builder methods return a new `Response`
- `bast.App` — entry point satisfying `http.Handler`; `New`, `Use`, `Guard`, `Register`, `Listen`, `Shutdown`
- `bast.Module` — portable, self-contained unit with `Prefix`, `Controller`, `Middleware`, `Guards`, nested `Modules`, and `Doc`
- `bast.HandlerFunc` / `bast.StreamHandlerFunc` — distinct handler signatures for request/response and streaming
- `bast.Guard` / `bast.GuardFunc` / `bast.SecuredGuard` — pre-handler checks with OpenAPI security scheme support
- `bast.MiddlewareFunc` — pipeline composition at registration time (not at request time)
- `bast.StreamCtx` — non-pooled streaming context embedding `context.Context`; `Send`, `Write`, `Flush`, `SetHeader`, `Closed`

### Router

- Radix tree router written from scratch — no third-party router wrapping
- Static, named param (`:name`), and wildcard (`*name`) segment types
- Static-wins-over-param priority enforced at insertion time
- Method-not-allowed returns 405 with `Allow` header, not 404
- Path params stored in pre-allocated `[8]Param` array inside `Ctx` — zero heap allocations on the hot path
- `ctx.Param()` is a linear O(N) scan — faster than a hash for N≤8 due to cache locality

### Middleware

- `middleware.RequestID` — crypto-random request ID, stored in Ctx and attached as `X-Request-ID`
- `middleware.Logger` — structured slog-based request logging
- `middleware.Recover` — catches user handler panics, returns 500
- `middleware.CORS` — full CORS with wildcard/specific origins, preflight, credentials, `Vary` header

### Error boundary

- `bast.BastError` — typed error with `Status`, `Code`, `Message`; consistent JSON envelope
- `bast.ValidationError` — field-level validation errors; auto-detected by the boundary for 422 responses
- `bast.DefaultErrorHandler` — handles `BastError`, `ValidationError`, unknown errors (500, no internal leak)
- Error constructors: `ErrNotFound`, `ErrUnauthorized`, `ErrForbidden`, `ErrBadRequest`, `ErrConflict`, `ErrUnprocessable`, `ErrInternal`
- Error code constants: `CodeNotFound`, `CodeUnauthorized`, `CodeForbidden`, `CodeValidation`, `CodeConflict`, `CodeInternal`

### Operational

- `bast.HealthConfig` — `/health` (liveness, always 200) and `/ready` (readiness, 200/503) with `CustomCheck`
- `bast.LoadConfig[T]()` — typed env config with `env`, `default`, `required`, `secret` struct tags; nested structs; all missing required fields reported in one error
- `bast.Logger` interface — pluggable; default implementation produces colored `[Bast]`-prefixed NestJS-style output
- `OnBoot`, `OnModuleRegistered`, `OnRouteRegistered`, `OnListening`, `OnShutdown`, `OnRequest`, `OnError`, `Info/Warn/Error/Debug`
- Lifecycle hooks via `bast.Hooks`: `OnInit` (forward order), `OnReady` (after port bound), `OnShutdown` (reverse order, timeout-isolated)

### OpenAPI / Swagger

- OpenAPI 3.0.3 spec generated from code at startup — no comment annotations, no codegen tools
- `bast.Doc` on routes — summary, description, tags, params, request body, response shapes, deprecated
- `bast.Body[T]()` — captures response type via reflection once at startup
- `bast.SecuredGuard` → auto-populates `securitySchemes`
- Module `Doc` → OpenAPI `tags` array
- Swagger UI served at configurable path via CDN

### Testing

- `basttest.NewCtx()` — builds a `*Ctx` outside the pool; `WithParam`, `WithQuery`, `WithHeader`, `WithMethod`, `WithPath`, `WithBody`, `WithRawBody`, `WithStore`, `WithIP`
- `basttest.NewApp()` — integration test harness via `httptest`; `Do`, `TestResponse`, fluent `Assertions`

### CLI

- `bast new <name>` — scaffolds a working Todo API with full CRUD and in-memory storage
- `bast generate module <name>` — generates 5 files: module, controller, service, repo, dto
- `bast generate guard <name>` — generates a guard file
- `bast generate service <name>` — generates a shared service file

### Benchmarks

| | ns/op | allocs/op |
|---|---|---|
| Router — static route | 29 | **0** |
| Router — param route | 38 | **0** |
| Ctx acquire + release | 21 | **0** |
| Ctx param lookup | 3.5 | **0** |
| App — full static request | 454 | 4 |
| App — full param request | 473 | 4 |

---

[Unreleased]: https://github.com/bastion-framework/bast/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/bastion-framework/bast/releases/tag/v0.1.0