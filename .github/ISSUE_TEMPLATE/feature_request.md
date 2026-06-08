---
name: Feature Request
about: Propose a new feature or improvement for Bast
title: "feat: "
labels: enhancement
assignees: ""
---

## Problem statement

<!-- What problem does this solve? Who has this problem and in what context? -->

## Proposed solution

<!-- Describe what you want to happen. Include an API sketch if possible. -->

```go
// Example of how the feature would be used:
```

## Alternatives considered

<!-- What other approaches did you consider? Why is this the best one? -->

## Does this belong in core?

Bast core is intentionally lean. Before requesting a feature, consider:

- Does it require a new external dependency? → probably a companion package
- Does it use reflection on the hot path? → not acceptable in core
- Does it add magic or implicit behavior? → not aligned with Bast's philosophy
- Would it be useful to 80%+ of Bast users? → good candidate for core

## Additional context

<!-- Links, prior art, related issues, etc. -->