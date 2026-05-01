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
- **Django-style ORM** — Models, QuerySets, `filter("age__gte", "18")`, 26 lookup types, migrations, soft deletes, row locking, bulk operations, audit trails, time-travel queries — all built in.

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
- **File System** — Read, write, copy, move, delete, walk directories.
- **OS** — Environment variables, process execution, platform info.
- **Path** — Cross-platform path manipulation.
- **Args** — CLI argument parsing.
- **Datetime/Time** — Date arithmetic, formatting, timestamps.
- **Logging** — Leveled logging (debug, info, warn, error).
- **Compression** — gzip/zlib compress and decompress.

### Validation
- **Built-in validators** — `is_email()`, `is_url()`, `is_ipv4()`, `is_ipv6()`, `is_hex()`, `is_json()`. No third-party library needed for basic input validation.

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

## The ORM

This is probably the most ambitious part of Desi. We took Django's ORM — the part that makes Django productive — and built it into the language runtime.

### What you get without any external package:
- **Model definitions** — `char_field`, `integer_field`, `uuid_field`, `decimal_field`, `json_field`, etc.
- **QuerySets** — Lazy, chainable, Django-style: `filter("name__icontains", "alice")`, `exclude`, `order_by`, `limit`, `annotate`, `group_by`, `having`.
- **26 lookup types** — `exact`, `contains`, `startswith`, `gt`, `gte`, `in`, `range`, `isnull`, `year`, `month`, `json_has`, and more.
- **Migrations** — Generate, apply, rollback, squash, dry-run, multi-app support.
- **inspectdb** — Reverse-engineer models from an existing database.
- **Relationships** — ForeignKey, OneToOneField, ManyToMany with auto-junction tables.
- **Inheritance** — Abstract, multi-table, and proxy models.
- **Managers** — Reusable named query scopes.
- **Signals** — Pre/post save/delete hooks.
- **Soft deletes** — `is_deleted` filtering with restore support.
- **Row locking** — `SELECT FOR UPDATE` with NOWAIT and SKIP LOCKED.
- **Dirty field tracking** — Only UPDATE what actually changed.
- **Audit trail** — Built-in history tracking, no plugin needed.
- **Time-travel queries** — Query the state of a record at any point in time.
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
- **Single binary output** — Your entire program, including the runtime, database drivers, crypto, and HTTP client, compiles into one static binary. `scp` it to a server and run it.
- **Cross-platform** — macOS (x86_64, arm64), Linux (x86_64, arm64), Windows (cross-compile from macOS/Linux).

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
