## Summary

<!-- What does this PR do? Why? Link to the related issue if one exists. -->

Closes #

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Performance improvement
- [ ] Refactor (no behavior change)
- [ ] Documentation
- [ ] Chore / dependency update

## Checklist

- [ ] All existing tests pass: `go test ./...`
- [ ] New behavior has tests (tests written before implementation where possible)
- [ ] `go vet ./...` is clean
- [ ] No new allocations on the hot path — benchmarks checked if touching router/Ctx/pipeline
- [ ] No new external dependencies (or linked issue with discussion)
- [ ] No `reflect` outside `docs.go` / `config.go`
- [ ] Exported names match the spec exactly
- [ ] Errors wrapped with `fmt.Errorf("bast: <context>: %w", err)`
- [ ] Commit messages follow conventional commits

## Benchmark results (if applicable)

<!-- Paste `benchstat` output here if your change touches hot-path code. -->

```
BenchmarkApp_StaticRoute   ...
BenchmarkCtx_AcquireRelease ...
```

## Notes for reviewers

<!-- Anything the reviewer should pay particular attention to. -->