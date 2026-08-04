# Changelog

All notable changes to Desi will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

- `unreachable()` builtin — panics if reached; documents impossible code paths (mirrors Rust's `unreachable!()`)
- `todo()` builtin — panics with "not implemented"; placeholder for incremental development
- `chr(n)` / `ord(s)` — Unicode codepoint ↔ string conversion
- `hex(n)` / `oct(n)` / `bin(n)` — integer formatting with `0x`/`0o`/`0b` prefix
- `abs(n)` — absolute value for `int` and `float`
- `round(n, digits)` — float rounding to decimal places
- `pow(base, exp)` — integer exponentiation
- `hash(value)` / `id(value)` — FNV-1a hash and pointer identity
- Default object representation: classes and structs print as `<TypeName at 0xADDR>` when no `__repr__` is defined
- Windows build support: `build.ps1` script for full clean build on Windows (runtime + all compiler tools)
- The database module — ORM, query builder, migrations, connection pool, and the PostgreSQL/MySQL/Redis drivers — now builds and runs on Windows, so `import db` works on all three platforms
- VS Code extension (`editors/vscode`) with native `.desi` syntax highlighting and LSP integration

### Fixed

- `elif` chains now compile correctly — all arms were previously silently dropped, leaving only the `if` and final `else`
- String return values no longer carry a spurious `\n` — `return "positive"` was producing `"positive\n"`, breaking string comparisons

---

## [0.1.0] - 2026-05-06

### Added

**Core Language**

- Variables with `let`, and `let mut` where the binding may be reassigned
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
