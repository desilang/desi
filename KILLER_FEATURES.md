# Why Desi?

If you're reading this, you're either a contributor or someone curious about what makes Desi different. This document covers the design philosophy, the features we're most proud of, and what we think makes Desi worth trying.

Desi isn't trying to replace Python, Rust, or Go. It's taking the best ideas from each and combining them into something that feels genuinely productive. We want you to write code that's readable, safe, fast, and ships as a single binary — without fighting your tools to get there.

---

## The Design Philosophy

Desi pulls from several languages, and we're upfront about it:

- **Python** — The syntax and readability. If you know Python, you can read Desi code.
- **Rust** — Memory safety without a garbage collector. Move semantics. `Option<T>` and `Result<T, E>` for error handling. Trait bounds on generics.
- **Elixir/OTP** — Supervisors with auto-restart. Structured concurrency. The "let it crash" philosophy.
- **Go** — Channels for message passing. `select` for multiplexing. Simple concurrency model.
- **Django** — The ORM. QuerySets, field lookups, migrations, managers, signals — all built into the language.
- **C/C++** — Raw performance. LLVM backend. Compiles to native machine code. `@extern` decorator for seamless C interop.
- **Elixir's Pipeline** — The `|>` operator for chaining function calls left-to-right.

The result is a language where you can write a web backend, talk to Postgres, validate user input, hash passwords, serve HTTP, and deploy — all from `import` statements, no `pip install`, no `cargo add`, no `go get`.

---

## Batteries-Included Standard Library

This is the part that surprises people. Everything below ships with the compiler. No package manager needed.

### Databases & ORM
- **PostgreSQL & MySQL** — Pure C wire protocol clients. No `libpq`, no `libmysqlclient`, no system dependencies.
- **SQLite3** — The full engine compiled into the runtime. `import sqlite3` and you're done.
- **Redis** — Pure RESP protocol client.
- **Django-style ORM** — Models, QuerySets, `filter("age__gte", "18")`, 26 lookup types, migrations, soft deletes, row locking, bulk operations, audit trails, historical audit queries — all built in.

### Web & Networking
- **HTTP client** — GET/POST/PUT/PATCH/DELETE with TLS. Headers, timeouts, JSON helpers.
- **HTTP server** — Built-in request routing and response handling.
- **WebSocket** — Client support for real-time communication.
- **TCP/UDP** — Low-level socket API via the `net` module.

### Security & Encoding
- **Crypto** — SHA-256, SHA-512, MD5, SHA-1, HMAC, bcrypt password hashing, secure token generation.
- **Base64/Hex** — Encoding and decoding.
- **TLS** — Built into the HTTP and networking layers.
- **UUID** — v4 generation.

### Data & Config
- **JSON** — Parse and generate. Works with dicts and lists.
- **TOML** — Config file parsing.
- **CSV** — Read and write.
- **Templates** — `{{key}}` substitution with HTML escaping.
- **Regular Expressions** — POSIX-compatible regex via `re`.

### System & I/O
- **File System** — Read, write, copy, move, delete, walk directories, temp files.
- **OS** — Environment variables, process execution, platform info.
- **Path** — Cross-platform path manipulation.
- **Process** — Run external commands, capture stdout/stderr, exit codes, timeout with auto-kill, signal sending.
- **Args** — CLI argument parsing with flags, subcommands, and help generation.
- **Shell** — Shell scripting helpers: pipe, glob, cd, mkdir.
- **Signal** — OS signal handling (`SIGINT`, `SIGTERM`) for graceful server shutdown.
- **Datetime/Time** — Date arithmetic, formatting, timestamps, Duration, Stopwatch.
- **Logging** — Leveled logging (debug, info, warn, error, fatal).
- **Compression** — gzip/zlib compress and decompress.

### Code Analysis
- **Runtime AST Library** — `import ast` lets you parse, walk, and analyze `.desi` source files at runtime. Build custom linters, code generators, and documentation tools — using the same parser the compiler uses internally.
- **Text Diffing** — `import diff` for unified diff output between strings. Useful for testing, version control tools, and content comparison.

### Validation
- **Built-in validators** — `is_email()`, `is_url()`, `is_ipv4()`, `is_ipv6()`, `is_hex()`, `is_json()`. No third-party library needed for basic input validation.

---

## Performance Advisor — The Compiler That Coaches You

Most compilers tell you *what's wrong*. Desi tells you *what's slow*.

The **Performance Advisor** is a built-in static analysis engine that detects algorithmic anti-patterns at compile time — zero runtime overhead, zero external tools, zero configuration.

### What it catches:

```
⚠ DPR0001 [perf/complexity]: O(n²) nested loop detected
  ╭─ src/process.desi:24:9
  │
24│         for item in items:
  │         ^^^^^^^^^^^^^^^^^^
  │
  = help: Consider using a dict/set for O(1) lookups, or precompute a mapping
```

- **O(n²) nested loops** — Flags `for > for` patterns with suggestions for hash-based alternatives.
- **String concatenation in loops** — Detects `str + str` inside hot loops, recommends `list.join()`.
- **Repeated collection lookups** — Catches `list.contains()` in loops, suggests `set` or `dict`.
- **Unbounded allocations** — Warns about growing collections inside loops without size hints.

### Three analysis levels:

```toml
# desi.mod
[diagnostics]
perf = "default"    # "relaxed", "default", or "strict"
```

- **relaxed** — Only the most obvious anti-patterns (O(n²) loops, string concat in loops).
- **default** — Anti-patterns plus style suggestions (unnecessary copies, missed pipeline opportunities).
- **strict** — Everything above plus micro-optimizations (arena hints, branch prediction suggestions).

### `@perf` decorator for runtime benchmarking:

```desi
import perf

@perf(iterations=1000)
def my_algorithm(data: list[int]) -> int:
    return data |> filter((x) => x > 0) |> sum()

# Output:
# ┌─ perf: my_algorithm ─────────────────┐
# │ Iterations: 1,000                    │
# │ Total:      45.2ms                   │
# │ Average:    45.2μs                   │
# │ Min:        42.1μs  Max: 128.3μs     │
# └──────────────────────────────────────┘
```

The `@perf` decorator is **zero-cost in production** — it's automatically stripped from release builds via the `strip_in_release` macro protocol property. No `#ifdef`, no build flags, no dead code.

### `desic perf` subcommand:

```bash
desic perf src/main.desi          # Run all @perf functions
desic perf --level=strict .       # Strict advisor on entire project
```

---

## Build Audit & Permissions — Security at Compile Time

Desi's compiler acts as a security gatekeeper. Before your code builds, the compiler scans all imports — including transitive dependencies — and reports exactly what sensitive APIs they use.

### `[permissions]` in desi.mod:

```toml
# desi.mod
[permissions]
allow = ["fs", "http", "json"]     # APIs your code is allowed to use
deny  = ["shell", "process"]       # Explicitly blocked — build fails if used
audit = true                       # Print audit report before building
```

### The audit report:

```
🔒 Build Audit Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Your code:
    ✓ fs       (Tier 1: System)
    ✓ http     (Tier 2: Network)
    ✓ json     (Tier 0: Safe)

  Third-party: analytics_lib v0.2.1
    ⚠ net      (Tier 2: Network) — used in analytics_lib/client.desi:17
    ⚠ process  (Tier 1: System) — used in analytics_lib/utils.desi:42
    ✗ DENIED: process is in deny list

  Build blocked. Remove 'process' from deny list or remove the dependency.
```

### API sensitivity tiers:

| Tier | Category | Modules | Risk |
|------|----------|---------|------|
| 0 | Safe | `math`, `string`, `json`, `re`, `collections` | None |
| 1 | System | `fs`, `os`, `path`, `env`, `args`, `process` | File/env access |
| 2 | Network | `http`, `net`, `db`, `redis`, `websocket` | Network access |
| 3 | Privileged | `unsafe` blocks, raw pointers, `@extern` FFI | Arbitrary code execution |

This isn't just a warning system — it's **enforceable policy**. CI pipelines can set `deny = ["shell", "unsafe"]` and guarantee no dependency introduces a backdoor.

---

## Memory Safety Without a Garbage Collector

Desi takes ideas from Rust but makes them more approachable:

- **Move semantics** — Values are moved by default. Use-after-move is a compile-time error.
- **Borrow checking** — The compiler tracks ownership and catches common bugs at compile time.
- **RAII via `using`** — Locks, files, database connections, task groups — all cleaned up automatically when the scope ends. The compiler enforces this: `let guard = mutex.lock()` is a compile error. You *must* use `using guard = mutex.lock():`.
- **Arena allocators** — Bulk-allocate, bulk-free. Great for request-scoped memory.
- **Reference counting** — `Rc[T]` for shared ownership when you need it.
- **No GC pauses** — Deterministic memory management means predictable latency.

```desi
using guard = mutex.lock():
    guard.value = guard.value + 1
# Lock is released here. Always. Even if something panics.
```

### Arena Allocators — Bulk Memory Without the Overhead

Arenas give you the speed of manual memory management with the safety of automatic cleanup:

```desi
def handle_request(req: Request) -> Response:
    using arena:
        # All allocations in this scope use the arena
        let header = arena.alloc(64)
        let body = arena.alloc(4096)
        let parsed = parse_json(body)

        return build_response(parsed)
    # Entire arena freed in ONE operation — no individual frees,
    # no GC tracing, no reference counting overhead
```

**Why this matters:**
- **Constant-time allocation** — Bump pointer, no free-list search.
- **Constant-time deallocation** — Free everything at once when the scope ends.
- **Cache-friendly** — Objects allocated together live together in memory.
- **Perfect for request handling** — Each HTTP request gets an arena, processes, and the entire arena is freed when the response is sent. No memory leaks possible.

---

## Concurrency That Doesn't Hurt

### From Go: Channels and Select
```desi
let ch = channel_new(10)
spawn async () => ch.send(42)
let val = ch.recv()

select:
    case msg = rx1.try_recv():
        print(f"Got: {msg}")
    default:
        print("Nothing ready")
```

### From Elixir/OTP: Supervisors
Processes that crash get restarted automatically. No manual error recovery:

```desi
using sup = sync.Supervisor():
    sup.start_child("worker_1", my_worker)  # Persistent — auto-restarts on crash
    sup.submit(one_off_task)                 # Pool task — runs once
    sup.stop()
```

### Structured Concurrency: TaskGroups
All spawned tasks must complete before the group ends. No leaked goroutines:

```desi
using tg = sync.TaskGroup():
    tg.run(process_chunk_1)
    tg.run(process_chunk_2)
    tg.wait()  # Both must finish
```

### Thread-Safe Primitives
Mutex, RwLock, Semaphore, Channels — all in `import sync`.

---

## User-Defined Macros — Extend the Compiler

Desi's macro system lets you define custom decorators that hook into the compiler pipeline. Macros are defined as `@macro` classes and registered at compile time — zero runtime overhead.

```desi
@macro(target="class")
class model:
    property_name = "objects"
    chainable = ["filter", "exclude", "order_by", "limit"]
    terminal = ["fetch_all", "fetch_one", "count", "exists"]
    runtime = {
        "filter": "__db_filter",
        "fetch_all": "__db_fetch_all"
    }
    require_fields = true
    auto_pk = true
```

This single class definition gives every `@model` class a full QuerySet API, wired to C runtime functions, with validation rules — all enforced at compile time.

**What macros can do:**
- Inject properties and methods into decorated classes/functions.
- Define chainable/terminal method APIs (like QuerySets).
- Wire methods to C runtime functions via the `runtime` mapping.
- Validate class structure (require fields, forbid names, auto-generate primary keys).
- Support `strip_in_release = true` for dev-only decorators (like `@perf`, `@test`).

**What's coming in v0.2.0:** Compile-time macro introspection — write macro rules in Desi itself, with full AST access. See the [roadmap](docs/roadmap/todo/compile_time_macros.md).

---

## The ORM

This is probably the most ambitious part of Desi. We took Django's ORM — the part that makes Django productive — and built it into the language runtime.

### What you get without any external package:
- **Model definitions** — `char_field`, `integer_field`, `uuid_field`, `decimal_field`, `json_field`, etc.
- **QuerySets** — Lazy, chainable, Django-style: `filter("name__icontains", "alice")`, `exclude`, `order_by`, `limit`, `annotate`, `group_by`, `having`.
- **26 lookup types** — `exact`, `iexact`, `contains`, `icontains`, `startswith`, `istartswith`, `endswith`, `iendswith`, `gt`, `gte`, `lt`, `lte`, `ne`, `in`, `range`, `isnull`, `year`, `month`, `day`, `hour`, `minute`, `second`, `quarter`, `week` — plus JSON lookups: `has`, `contains`.
- **Migrations** — Generate, apply, rollback, squash, dry-run, multi-app support.
- **inspectdb** — Reverse-engineer models from an existing database.
- **Relationships** — ForeignKey, OneToOneField, ManyToMany with auto-junction tables.
- **Inheritance** — Abstract, multi-table, and proxy models.
- **Managers** — Reusable named query scopes.
- **Signals** — Pre/post save/delete hooks.
- **Soft deletes** — `is_deleted` filtering with restore support.
- **Row locking** — `SELECT FOR UPDATE` with NOWAIT and SKIP LOCKED.
- **Dirty field tracking** — Only UPDATE what actually changed.
- **Audit trail** — Built-in history tracking with historical queries. No plugin needed — `db.enable_audit("users")` auto-creates a history table and logs every change.
- **Field validation** — Runtime "did you mean?" suggestions for typo'd field names using Levenshtein distance.

```desi
import db

db.connect("postgres", "localhost", 5432, "mydb", "user", "pass")

db.objects("users")
db.filter("age__gte", "18")
db.filter("name__icontains", "alice")
db.order_by("-created_at")
db.limit(10)
let rows = db.fetch_all()
```

---

## Seamless C Interop

Desi compiles to LLVM IR, which means calling C code is trivial. The `@extern` decorator handles the bridging:

```desi
# Call any C function directly
@extern("C")
pub def strlen(s: str) -> int

# Safe wrapper — auto-generates unsafe block around the raw call
@extern("C", safe=true, c_name="abs")
pub def safe_abs(x: int) -> int
```

This makes it easy to wrap existing C libraries, call system APIs, or integrate with legacy codebases. The entire Desi runtime is written in C and exposed to Desi code through this mechanism.

---

## Type System

- **Strong static typing** with inference — you rarely need to annotate.
- **Generics** with trait bounds: `def sort<T: Ord>(items: list[T])`.
- **Turbofish syntax** for explicit type arguments: `identity::<int>(42)`.
- **Option\<T\>** and **Result\<T, E\>** — No null. No exceptions. Errors are values.
- **Pattern matching** with `match` expressions — exhaustive, with destructuring.
- **Enums** with associated data (algebraic data types).
- **Dunder methods** — `__add__`, `__repr__`, `__getitem__`, `__format__`, etc. for operator overloading.

```desi
match result:
    Result.Ok(value): print(f"Got: {value}")
    Result.Err(msg): print(f"Error: {msg}")
```

---

## Developer Experience

- **REPL** — `desirepl` for interactive exploration.
- **Formatter** — `desifmt` for consistent code style.
- **LSP** — Full language server with hover, completions, go-to-definition, rename, code actions, semantic highlighting, inlay hints. Works in VS Code, IntelliJ, and any LSP-compatible editor.
- **Performance advisor** — The compiler warns you about slow patterns before you even run your code.
- **Build audit** — Know exactly what APIs your dependencies use before building.
- **Single binary output** — Your entire program, including the runtime, database drivers, crypto, and HTTP client, compiles into one static binary. `scp` it to a server and run it.
- **Cross-platform** — macOS (x86_64, arm64), Linux (x86_64, arm64), Windows (cross-compile from macOS/Linux).

---

## Hot Reload (Development Mode)

Desi supports **hot reload in development** using native compilation + dynamic linking. No VM, no interpreter, no bytecode — just fast incremental recompilation:

```
Developer makes edit →
  1. File watcher detects change
  2. Recompile only affected modules to .so/.dylib
  3. Pause running program at safe point
  4. dlclose old module, dlopen new module
  5. Rewire function pointers in dispatch table
  6. Resume execution
```

- **Sub-second reload** for typical edits via LLVM incremental compilation.
- **State preservation** via `write_state()`/`read_state()` hooks.
- **Production builds are fully static** — no dynamic linking overhead.

---

## What's Coming

- **Distributed Systems (v0.2.0)** — Erlang-inspired node connection, cross-node messaging, and distributed supervisors. Built on Desi's existing concurrency primitives with a binary wire protocol over TCP. See the [design doc](docs/roadmap/todo/distributed_systems.md).
- **Compile-time Macros (v0.2.0)** — Write macro rules in Desi itself with full AST access. Community-contributed analyzer rules without modifying the Go compiler.
- **Full Async/Await (v0.2.0)** — Design doc exists, implementation planned.
- **WebAssembly Target (v0.2.0)** — Compile Desi to WASM for browser and edge deployments.

---

## What Desi Is Not

We want to be honest about where Desi is today:

- **Not a replacement for Python's ecosystem** — We don't have numpy, pandas, or ML libraries. Desi is for backend services, CLI tools, and systems work.
- **Not battle-tested in production** — This is v0.1.0. We're looking for early adopters and contributors who want to shape the language.
- **Not a web framework** — We have HTTP client/server, templates, and an ORM, but we're not Django or Rails. We're the *foundation* you'd build a framework on.

---

## Getting Involved

If any of this sounds interesting, we'd love your help:

- **Try it** — Build something small. File bugs. Tell us what's confusing.
- **Read the book** — [desilang.org](https://desilang.org) has full documentation.
- **Contribute** — The compiler is written in Go. The runtime is C. Both are straightforward to hack on.
- **Talk to us** — Open an issue, start a discussion, or just say hi.

The goal is to make a language that's genuinely useful — not a toy, not a research project, but something you'd actually want to write real software in.
