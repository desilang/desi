# Changelog

All notable changes to Desi will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.1.0] - Unreleased

### Added

**Core Language**

- Variables with `let`, and `let mut` for the ones that change
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
- `try/except/finally/raise` statements with Python-style syntax
- Catchable exceptions via `setjmp`/`longjmp` runtime (10 built-in exception types)
- `TryExpr` (`?` operator) context-aware: redirects to except handler inside try blocks

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
- `strings` module: `strip`/`lstrip`/`rstrip` Python-compatible aliases for `trim`

**Performance Tooling**

- `desic perf` subcommand with `--level=relaxed|default|strict`
- Performance advisor rules: `DPR0001` (nested loops), `DPR0002` (string concat in loop), `DPR0003` (repeated lookup), `DPR0004` (unbounded alloc)
- `@perf` decorator with `strip_in_release = true` (zero-cost in production)
- `runtime/perf.c` — high-resolution timing (mach_absolute_time / CLOCK_MONOTONIC)
- `import perf` — safe Desi wrappers for benchmarking
- `import ast` — runtime AST parsing and walking for `.desi` files

**Build Audit & Permissions**

- `[permissions]` section in `desi.mod` with `allow`, `deny`, `audit` keys
- API sensitivity tiers: Safe (0), System (1), Network (2), Privileged (3)
- Automatic build audit during `desic build` — blocks on denied imports
- TTY and JSON audit report output
- Deny-by-category support (e.g., `deny = ["privileged"]`)

**Tooling**

- `desic` — Compiler with `run`, `build`, `test`, `check`, `fmt`, `doc`, `emit-ir`, `init`, `watch`, `perf` commands
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

**Windows Platform Support**

- Full Windows build via `build.ps1` (MSVC runtime + Go tools) and
  `build-desi.ps1`/`test_examples.ps1` (program builds and test harness)
- Runtime modules ported to Win32: `os`, `net` (WinSock2), `fs`, `path`
  (dirent shim, `_fullpath`), `random`, `uuid` (`rand_s`), `process`
  (CreateProcess with pipes, timeouts, kill), `shell` (`_popen`,
  FindFirstFile glob), `re` (bundled minimal-ERE regex shim), hot-reload
  state functions
- HTTPS client on Windows via Schannel (`http/tls_win.h`) — no OpenSSL
  needed; HTTP/WebSocket *server* and `signal` remain macOS/Linux-only
- 479 of 487 examples pass on Windows (remaining 8 need the db/ORM
  runtime port)

**Release builds and benchmarks**

- `desic build --release` / `desic run --release` (alias for `-O2`):
  the emitted IR runs the full clang -O2 pipeline; on Unix, optimized
  builds route through clang instead of llc so inlining and loop
  optimizations apply. `build-desi.ps1 -Release` / `build-desi.sh
  --release` (or `DESI_RELEASE=1`) for script builds
- Cross-module LTO for release builds (Windows): build.ps1 compiles a
  benchmark-curated hot set of runtime files (recursion guard, dict,
  strings, rc, arena) to LLVM bitcode; release links inline them into
  user code via -flto -fuse-ld=lld. The recursion guard's cold path is
  outlined so inlining it costs nothing per frame
- Allocator overhaul for collections: dict entries are pooled in
  64-entry blocks with values <= 8 bytes stored inline (an insert that
  cost 2 mallocs now costs ~1/64th of one), and a list's header and
  initial capacity share a single malloc. The built-in dict now
  matches or beats a hand-rolled C open-addressing table on time and
  memory
- Owned string accumulators (hybrid MM phase 4, first step): when the
  compiler proves a mutable string local is only used in borrowing
  positions, `let mut s = "lit"` becomes a heap copy and `s := s + x`
  lowers to a realloc-based append that frees the old value. The
  build-a-string loop went from 43.5 MB leaked (O(n^2) copying) to a
  1.6 MB flat working set — half of C's amortized buffer. Unprovable
  cases keep the leak-safe lowering
- `benchmarks/`: eight paired Desi-vs-C programs (identical work, same
  clang opt level, outputs must match) with time + peak-memory runners
  for Windows and Unix. At -O2+LTO Desi beats C on dict operations and
  integer loops, trades wins on string churn, and ties matrix multiply;
  the README documents every remaining delta and its planned fix

**Automatic Memory Management (hybrid MM, phases 1–3)**

- Collection locals (`list`/`set`/`dict`) are freed at scope exit; float
  element boxes are list-owned (freed on `list_free`, cloned across
  copy/slice/extend/filter)
- Enum instances (wrapper + payload box) and struct instances are freed
  at scope exit with recursive heap-field cleanup
- String temporaries are freed at scope exit: `+` concatenation,
  f-strings, `str(int/float/bool)`, `replace()`/`join()` results, and
  `print`'s collection-to-string conversions no longer leak when used
  transiently — `print("n: " + str(i))` in a hot loop now runs in
  constant memory. Ownership transfers on binding, return, call
  arguments, collection inserts, construction, and channel sends.
- Conservative ownership analysis in the lowerer: values passed to user
  functions, stored in containers, matched with payload bindings, or
  aliased via field access/`?` extraction are never double-freed —
  unclear ownership leaks safely instead of crashing
- Constant-memory loops: millions of list/enum/string-temp allocations
  peak at ~3 MB working set

### Fixed

- All 475 examples pass (0 failures)
- Cross-platform LLVM tool auto-discovery for `llc`/`clang` (macOS Homebrew, Linux versioned, Windows MSYS2/Chocolatey)
- Makefile: Fixed archive merge that lost libmpdec symbols
- Guard pattern checker prevents guard/arena escapes
- `using tg = sync.TaskGroup():` destroyed the TaskGroup as an arena —
  heap corruption on every platform (crashed deterministically on
  Windows; macOS survived only by memory-layout luck)
- Windows x64 bool ABI: `i1` arguments passed to C `int` parameters left
  garbage in the upper register bits — f-string bools always printed
  "true" and `assert_eq(1 < 2, true)` failed while printing
  "expected: true, actual: true" (`bool_to_str`, `bool_to_cstring`,
  `__desi_assert_{eq,ne}_bool` all zext'd now)
- `file_read_all` left the output slot uninitialized on error paths —
  reading a failed `open()` crashed instead of returning ""
- Struct-field drop offsets now match the aligned construction layout
  (`{id: int, status: Enum}` stores the pointer at offset 8, not 4)
- Enum unit-variant constructors store a full 8-byte null payload slot
  (was a 4-byte store leaving garbage in the upper half)
- F-strings in long loops crashed with a stack overflow: the lowering
  alloca'd an out-parameter slot per evaluation and LLVM only reclaims
  allocas on function return. F-strings now compile to a single
  `__desi_sprintf` call that returns the malloc'd string
- `rc`/`arc` reference counts are now atomic (MSVC interlocked
  intrinsics / GCC-Clang `__atomic` builtins) — `arc[T]` shared across
  threads no longer races the refcount; `weak.upgrade()` uses a CAS loop
  and the header free follows the collective-weak scheme, eliminating
  the `__rc_dec`/`__weak_dec` double-free race

### Tests

- 24 tests for performance advisor, build audit, and API tier classification

---

## Version History

| Version | Release Date | Highlights |
|---------|--------------|------------|
| 0.1.0 | Unreleased | Initial release — batteries-included systems language |

---

## Upgrade Guides

### Upgrading to 0.2.0 (Future)

!!! note "Coming Soon"
    Upgrade guide will be added when 0.2.0 is released.

When upgrading between major versions, check this section for breaking changes and migration steps.
