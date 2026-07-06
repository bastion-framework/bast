# Changelog

All notable changes to Bast are documented here.

This project follows [Semantic Versioning](https://semver.org) and the format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [v0.4.0] — 2026-07-06
### Added

- *(cli)* Add run and build commands to bast ([`71beae5`](https://github.com/bastion-framework/bast/commit/71beae53d5dde92088b079f5fcd8e2d55a6cc3a0))
### Documentation

- Update docs' ([`a3cf6a5`](https://github.com/bastion-framework/bast/commit/a3cf6a55b86ae6477fee112a57cf1a2f5f6d9dc4))
- Update: ([`0af19c4`](https://github.com/bastion-framework/bast/commit/0af19c49122c50e4dbd56ba6146c7681a428d2ff))
### Fixed

- Safe server defaults, dead config fields, builtin panic recovery, trustedproxies fails loudly, stream status logging ([`6199894`](https://github.com/bastion-framework/bast/commit/6199894befb6759935d646715c428289ce7d94cd))
- Relative redirect panic, BindForm no-op ([`5dfff1f`](https://github.com/bastion-framework/bast/commit/5dfff1f72b92de450b965c6dc6daad86efdea4f7))
- Xff spoofing, readiness leak, sse injection, withValue aliasing, cors ([`5cc57cf`](https://github.com/bastion-framework/bast/commit/5cc57cf37bff5fb744bd6848eaeb9178300d545c))
- Add auto-OPTIONS, docs disable option ([`71a07e3`](https://github.com/bastion-framework/bast/commit/71a07e3d0ee5fee6dfca79e65b97fb64642e67f8))

---
## [v0.3.0] — 2026-06-23
### Added

- Full re-write for the bast router ([`4b7236a`](https://github.com/bastion-framework/bast/commit/4b7236a0e13d7b31baf6aa7b7ce676285af29e03))

---
## [v0.2.1] — 2026-06-11
### Fixed

- *(stream)* Strean route guard + path param gaps ([`686ba5e`](https://github.com/bastion-framework/bast/commit/686ba5ecbf6118148af2a0ecf4712894a14dffc6))
- Address some risk issues ([`1d6629a`](https://github.com/bastion-framework/bast/commit/1d6629a739526bc48ff9b879a56c10fe006be3a2))

---
## [v0.1.1] — 2026-06-08
### Added

- *(core)* Add core types, Ctx, StreamCtx, and module skeleton ([`ecb5042`](https://github.com/bastion-framework/bast/commit/ecb504231bb3c0b5a3a26cc372bb50f2cdede2b9))
- *(router)* Add radix tree router ([`868f135`](https://github.com/bastion-framework/bast/commit/868f135748b8c66085725d7439b4baa6d738ce18))
- *(basttest)* Add bastest, a builtin test package for bast ([`a3cc4e9`](https://github.com/bastion-framework/bast/commit/a3cc4e9facf5743533c9816f7c79196e31fdbc03))
- *(middleware)* Add functional middlewares ([`c751a5e`](https://github.com/bastion-framework/bast/commit/c751a5e8e4d452c880d0d1deef061acd86aded31))
- *(health)* Add healthy and ready support functionalities ([`c766b05`](https://github.com/bastion-framework/bast/commit/c766b0524a77560713b303bcd06be7a3aaa8ff04))
- *(docs)* Add openapi,swagger docs support ([`cab1439`](https://github.com/bastion-framework/bast/commit/cab14395d222f4d93c9348cecff540b3b38d1d0d))
- *(cli)* Add cli support to bast ([`89c220b`](https://github.com/bastion-framework/bast/commit/89c220be57bffa51d3db34057eab00c1947c3fb6))
- Add benches ([`47f1885`](https://github.com/bastion-framework/bast/commit/47f1885c91e4a659a1938c49597af5b132998b34))
- Add benchmarks, fuzz tests, and zero-alloc router ([`a6b374c`](https://github.com/bastion-framework/bast/commit/a6b374cf278d8bad790ab46d3e5fbf0c49d9be75))
- *(docs)* Add bast documentation ([`5561419`](https://github.com/bastion-framework/bast/commit/556141965c6b5ea37e384dfbf64dac948ed469eb))
### Documentation

- Update docs via ci ([`4816525`](https://github.com/bastion-framework/bast/commit/4816525ad00d7eff780fe44c828ed86f74471cd3))
- Hope you got to work ([`e196816`](https://github.com/bastion-framework/bast/commit/e196816bbbdc09874cc59700324faa5a56a94d61))
### Fixed

- *(ci)* Go.mod was picking up path used in smottest experiment ([`aec31a3`](https://github.com/bastion-framework/bast/commit/aec31a3ada22cadd017075dfdc27d3c195be2b36))
- *(cli)* Fix issues in project scafould and error reporting binding ([`f42606c`](https://github.com/bastion-framework/bast/commit/f42606c83412f64cd0dad4a2e904ce57d81f9fde))
- *(docs)* Update docs to use ci docs build ([`bfc4d3e`](https://github.com/bastion-framework/bast/commit/bfc4d3e3b8dcb628c5840043a8e19bc79cec3f3c))
- Docs ([`a711cae`](https://github.com/bastion-framework/bast/commit/a711cae9a8cdf1409d14fb4927cc360671da2fc6))

---

