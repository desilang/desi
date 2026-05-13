# Changelog

All notable changes to Desi will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

**Error Handling (M15)**
- `try/except/finally/raise` statements with Python-style syntax
- Catchable exceptions via `setjmp`/`longjmp` runtime (10 built-in exception types)
- `TryExpr` (`?` operator) context-aware: redirects to except handler inside try blocks
- `__desi_panic(msg)` runtime function

**Performance Tooling (M16)**
- `desic perf` subcommand with `--level=relaxed|default|strict`
- Performance advisor rules: `DPR0001` (nested loops), `DPR0002` (string concat in loop), `DPR0003` (repeated lookup), `DPR0004` (unbounded alloc)
- `@perf` decorator with `strip_in_release = true` (zero-cost in production)
- `runtime/perf.c` — high-resolution timing (mach_absolute_time / CLOCK_MONOTONIC)
- `import perf` — safe Desi wrappers for benchmarking
- `import ast` — runtime AST parsing and walking for `.desi` files

**Build Audit & Permissions (M17)**
- `[permissions]` section in `desi.mod` with `allow`, `deny`, `audit` keys
- API sensitivity tiers: Safe (0), System (1), Network (2), Privileged (3)
- Automatic build audit during `desic build` — blocks on denied imports
- TTY and JSON audit report output
- Deny-by-category support (e.g., `deny = ["privileged"]`)

### Tests
- 24 new tests for performance advisor, build audit, and API tier classification

---

## [0.1.0] - 2026-05-06

### Added

**Core Language**

- Variables with `let` (immutable) and `var` (mutable)
- Type inference with optional annotations
- Functions with `def`, lambdas, closures
- Classes with inheritance, static/class methods, operator overloading
- Structs and enums with pattern matching (`match`)
- Generics with trait bounds and turbofish syntax
- Pipeline operator (`|>`)
- Membership operator (`in`)
- Raw strings (`r"..."`)
- Global constants
- Safe FFI with `extern` decorator
- `assert` statement

**Type System**

- Basic types: `int`, `float`, `str`, `bool`
- Sized integers: `i32`, `i64`, `u32`, `u64`
- Collections: `list<T>`, `dict<K,V>`, `set<T>`
- Generic classes and functions
- `Option<T>` and `Result<T,E>` types
- Tuples, slicing, iterators

**Builtins**

- I/O: `print()`
- Collections: `len()`, `range()`, `enumerate()`
- Aggregation: `sum()`, `min()`, `max()`
- Boolean: `any()`, `all()`
- Transformation: `map()`, `filter()`, `reduce()`
- Ordering: `sorted()`, `reversed()`
- Combining: `zip()`

**Memory Management**

- Reference counting
- Arena allocators
- RAII with `using`
- Guard pattern lifetime checker

**Standard Library**

- Core: `strings`, `os`, `io`, `fs`, `path`, `sys`, `args`, `env`
- Data: `json`, `toml`, `csv`, `fmt`, `template`, `encoding`, `base64`, `compress`
- Math: `math`, `random`
- Time: `time`, `datetime`
- Network: `http` (client), `http_server` (server with routing, CORS, rate limiting, static files), `net`
- WebSocket: Full RFC 6455 support with rooms, broadcast, permessage-deflate, WSS
- Crypto: `crypto`, `hash`, `uuid`
- Database: `db` (ORM), `sqlite3`, `postgres`, `mysql`, `redis`, `migrations`
- Concurrency: `sync` (Mutex, RwLock, Channel, TaskGroup, Supervisor, Semaphore, Atomic, Select)
- Utilities: `log`, `re`, `color`, `validate`

**Tooling**

- `desic` — Compiler with `run`, `build`, `test`, `check`, `fmt`, `doc`, `emit-ir`, `init`, `watch` commands
- `desifmt` — Code formatter (idempotent, golden-tested)
- `desirepl` — Interactive REPL with parse + type-check feedback
- `desilsp` — Full Language Server Protocol implementation (hover, completion, definitions, references, rename, semantic tokens, inlay hints, call hierarchy, workspace symbols)

**Self-Contained Runtime**

- Bundled miniz 3.0.2 — no system zlib (`-lz`) dependency
- Embedded SHA-1/SHA-256/SHA-512/MD5/HMAC — no OpenSSL for hashing
- Embedded SQLite3 — zero external database dependencies
- Embedded libmpdec — arbitrary precision decimals
- Only external dependency: OpenSSL (for TLS/HTTPS only)

**Documentation**

- Complete "Desi Book" with MkDocs Material:
  - Getting Started (install, first program, IDE setup)
  - 6 tutorials (variables, functions, collections, classes, generics)
  - 20+ language guide pages
  - 30+ stdlib reference pages
  - Concurrency guide (12 pages)
  - Examples index

### Fixed

- All 461 examples pass (0 failures)
- Makefile: Fixed archive merge that lost libmpdec symbols
- Guard pattern checker prevents guard/arena escapes

---

## Version History

| Version | Release Date | Highlights |
|---------|--------------|------------|
| 0.1.0 | 2026-05-06 | Initial release — batteries-included systems language |

---

## Upgrade Guides

### Upgrading to 0.2.0 (Future)

!!! note "Coming Soon"
    Upgrade guide will be added when 0.2.0 is released.

When upgrading between major versions, check this section for breaking changes and migration steps.
