# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | ✅ Yes    |

Only the latest minor release receives security fixes.

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report security issues by email to:

**kasiimlyee@gmail.com**

Include in your report:

- A description of the vulnerability and its potential impact
- Steps to reproduce or a minimal proof-of-concept
- The affected version(s)
- Any suggested fix if you have one

You will receive an acknowledgement within **48 hours** and a status update within **7 days**.

## Disclosure Policy

- Security fixes are released as patch versions (e.g. `v0.1.1`)
- A GitHub Security Advisory is published at the time of the fix
- Credit is given to the reporter in the advisory unless anonymity is requested

## Scope

The following are in scope:

- Request smuggling or routing bypass in the radix tree router
- `sync.Pool` misuse leading to data leakage between requests
- `ctx.IP()` trusted proxy bypass
- Header injection via response builder methods
- Panics in framework code reachable from user input (as opposed to user handler panics, which `middleware.Recover` is designed to catch)

The following are **out of scope**:

- Vulnerabilities in user application code built on top of Bast
- Vulnerabilities in companion packages (`bast-pgx`, `bast-gorm`, etc.) — report those to their respective repos
- Denial of service via unbounded request body size when `MaxBodySize` is explicitly disabled by the user