# Contributing to Bast

Thank you for your interest in contributing. Bast is a production framework, every contribution is held to a high standard because real applications depend on it. Please read this guide fully before opening a pull request.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Where to Start](#where-to-start)
- [Development Setup](#development-setup)
- [Implementation Rules](#implementation-rules)
- [Commit Style](#commit-style)
- [Pull Request Process](#pull-request-process)
- [PR Checklist](#pr-checklist)
- [Reporting Bugs](#reporting-bugs)
- [Requesting Features](#requesting-features)

---

## Code of Conduct

All contributors are expected to follow our [Code of Conduct](CODE_OF_CONDUCT.md). Be respectful and constructive.

---

## Where to Start

| Type | Where |
|---|---|
| Bug report | [GitHub Issues](https://github.com/bastion-framework/bast/issues) using the bug template |
| Feature request | [GitHub Issues](https://github.com/bastion-framework/bast/issues) using the feature template |
| Discussion / question | [GitHub Discussions](https://github.com/bastion-framework/bast/discussions) |
| Code contribution | Fork → branch → PR against `main` |

If you want to work on something non-trivial, **open an issue first** and discuss the approach before writing code. This avoids wasted effort if the direction doesn't fit the framework's philosophy.

---

## Development Setup

```bash
# Fork and clone
git clone https://github.com/<your-fork>/bast.git
cd bast

# Verify everything works
go test ./...

# Run benchmarks
go test -bench=. -benchmem -count=3 -run=^$ ./...

# Run fuzz tests (short mode — CI-safe)
go test -fuzz=FuzzRouter_Find -fuzztime=10s ./fuzz/
```

**Requirements:**
- Go 1.22 or later
- No external tools beyond the Go toolchain

---

## Implementation Rules

These rules are non-negotiable. PRs that violate them will not be merged.

### 1. Tests before implementation
Write the test first. The test is the spec in code. No new exported symbol ships without a test.

### 2. Zero allocations on the hot path
The hot path is: request received → `*Ctx` acquired → handler called → `Response` written → `*Ctx` released.

`BenchmarkCtx_AcquireRelease` and all `BenchmarkRouter_*` **must remain at 0 allocs/op**. Run benchmarks before and after your change:

```bash
go test -bench=. -benchmem -count=5 -run=^$ ./... > bench_head.txt
benchstat bench_base.txt bench_head.txt
```

A regression above 5% on `BenchmarkApp_*` requires justification in the PR description.

### 3. No new dependencies without approval
Check `go.mod` before any `go get`. The dependency list is a contract. Open an issue to discuss a new dependency before adding it.

### 4. No reflection outside `docs.go` and `config.go`
Both of those files use reflection once at startup. If you think you need reflection elsewhere, you are solving the wrong problem.

### 5. `*Ctx` must never implement `context.Context`
Do not add `Deadline()`, `Done()`, `Err()`, or `Value()` methods to `*Ctx`. This is structural safety — the compiler prevents accidentally passing a pooled `*Ctx` to a goroutine.

### 6. `Response` is a value type
All methods on `Response` use value receivers and return a new `Response`. Never mutate in place.

### 7. Errors must always carry context
Every `return err` in framework code must be:
```go
return fmt.Errorf("bast: <where>: %w", err)
```
Never swallow errors silently.

### 8. No `init()` functions
Initialization order must be explicit. Two `App` instances in the same process must share no global state (except `sync.Pool`).

### 9. Exported names match the spec
Do not rename types or methods. The spec is the API contract. Deviations require a `// DEVIATION:` comment with a justification.

---

## Commit Style

Use conventional commits:

```
feat(ctx): add WithValue for middleware value propagation
fix(router): handle trailing slash in wildcard routes
perf(router): replace param map with pre-allocated [8]Param array
test(basttest): add WithIP option for IP spoofing tests
docs: update BENCH.md with v0.1.1 baselines
chore: bump Go version requirement to 1.23
```

**Types:** `feat`, `fix`, `perf`, `test`, `docs`, `refactor`, `chore`

Keep the subject line under 72 characters. Use the body to explain _why_, not _what_ — the diff shows the what.

---

## Pull Request Process

1. **Fork** the repository and create a branch from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```

2. **Write tests first.** Your PR should include tests before the implementation, or at minimum alongside it.

3. **Run the full suite** before pushing:
   ```bash
   go test ./...
   go vet ./...
   ```

4. **Check benchmarks** if your change touches any hot-path code (router, ctx, middleware pipeline):
   ```bash
   go test -bench=. -benchmem -count=3 -run=^$ ./...
   ```

5. **Open the PR** against `main` using the pull request template.

6. **Address review feedback** promptly. PRs with no activity for 30 days will be closed.

PRs that touch the router, Ctx pool, or the request lifecycle will receive extra scrutiny — that code runs on every request for every user.

---

## PR Checklist

Before submitting, confirm:

- [ ] All existing tests pass: `go test ./...`
- [ ] New behavior has tests
- [ ] `go vet ./...` is clean
- [ ] No new allocations on the hot path (benchmarks checked)
- [ ] No new external dependencies (or discussed in an issue first)
- [ ] No `reflect` outside `docs.go` / `config.go`
- [ ] Exported names match the spec
- [ ] Errors wrapped with `fmt.Errorf("bast: ...: %w", err)`
- [ ] Commit messages follow the conventional commit style

---

## Reporting Bugs

Open an issue using the **Bug Report** template. Include:

- Go version (`go version`)
- OS and architecture
- Minimal reproducible example
- Actual vs expected behavior

---

## Requesting Features

Open an issue using the **Feature Request** template. Include:

- The use case — what problem does it solve?
- How it fits the framework's philosophy (explicit, lean, no magic)
- Whether it belongs in the core or as a companion package

Features that add external dependencies, use reflection on the hot path, or violate the framework's philosophy will not be accepted into core. They may be suitable as companion packages under the `bastion-framework` org.

---

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).