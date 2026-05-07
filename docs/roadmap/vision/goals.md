# Desi Language: Vision & Roadmap

**Last Updated:** May 07, 2026
**Status:** Active Development (Approaching v0.1.0)

## 🌟 The Vision

Desi is built on a simple premise: **Write like Python, Run like Rust.**

We want the ergonomics of a dynamic language with the performance and safety of a systems language. We are building a compiler that gives you control without the headache—memory safety without a garbage collector, and speed without complex syntax.

---

## ✅ Current Features (v0.1.0-alpha)

This is what works **right now**. The compiler is already capable of handling complex logic, generics, and object-oriented patterns.

### 1. Type System & Generics
-   **Static Typing**: Strong, compile-time type checking.
-   **Generics**: true monomorphization-ready generics (`Option<T>`, `Result<T, E>`, `list<T>`).
    -   *Verified:* Arbitrary nesting (`list<list<int>>`).
    -   *Verified:* Generic functions (`def identity<T>(x: T) -> T`).
    -   *Verified:* Generic enums with payload variants.
-   **Union Types**: `int | str` support (currently lowering to tagged unions or Any).
-   **Primitive Types**: `int`, `float`, `bool`, `str`, `none`.

### 2. Memory Management (Tier-0)
-   **RAII / Drop**: Deterministic destruction. Objects are freed when they go out of scope.
-   **Move Semantics**: Assigning a value moves ownership (preventing double-free bugs).
-   **Borrowing**: explicit `ref` and `inout` parameters for pass-by-reference.
-   **Arena Allocators**: Bulk-allocate, bulk-free for request-scoped memory.
-   **Reference Counting**: `Rc[T]` for shared ownership, `Arc[T]` for thread-safe shared ownership.
-   **No GC**: currently using `malloc`/`free` under the hood with compiler-inserted destructors.

### 3. Object-Oriented Programming (Classes)
-   **Classes**: `class Point:` with fields and methods.
-   **Visibility**: `pub` for public API, private by default.
-   **Mutability**:
    -   `let mut x` vs `let x`.
    -   Per-field mutability: `pub mut x: int`.
-   **Dunder Methods** (Operator Overloading):
    -   Arithmetic: `__add__`, `__sub__`, `__mul__`, `__div__`
    -   Comparison: `__eq__`, `__lt__`, etc.
    -   Bitwise: `__and__`, `__or__`, `__lshift__`, etc.
    -   indexing: `__getitem__`, `__setitem__`
    -   Stringify: `__str__`, `__repr__`
-   **Inheritance**: Single inheritance supported (`class Dog(Animal)`).
-   **Static Fields**: `pub static count: int`.

### 4. Control Flow & Pattern Matching
-   **Match Expressions**: Exhaustive pattern matching on Enums and values.
-   **Loops**: `while` and `for` (iterators).
-   **Conditionals**: `if`, `elif`, `else`.

### 5. Collections (Built-in)
-   `list<T>`: Dynamic arrays.
-   `dict<K, V>`: Hash maps.
-   `set<T>`: Unique item collections.
-   *Note:* Currently working on full generic support for these.

### 6. Extensible Macro System
-   **Declarative macros**: `@macro` class declarations define custom decorators.
-   **Protocol-based**: `MacroProtocol` with `OnCollect`, `OnCheck`, `OnLower` lifecycle hooks.
-   **Chainable/terminal APIs**: Define QuerySet-style method chains wired to C runtime functions.
-   **Validation rules**: `require_fields`, `auto_pk`, `forbidden_field_names`.
-   **Dev-only decorators**: `strip_in_release = true` for `@perf`, `@test`, etc.

---

## 🚀 Roadmap to v0.1.0 ("The MVP")

**Goal:** A stable, production-ready compiler that can build real CLI tools and basic applications. We are prioritizing stability over experimental features.

### 1. Fix Critical Gaps
-   [ ] **Recursion Limit**: Implement a default stack depth limit (e.g., 1000) to prevent segfaults on infinite recursion.
-   [ ] **Inheritance Workarounds**: Document and strictly define limitations on generic inheritance (e.g., `class Derived(Base<int>)` is deferred).

### 2. Standard Library (The "Thin Wrapper" Strategy)
We will not re-invent the wheel yet. v0.1.0 stdlib will be safe Desi wrappers around proven C functions.
-   [ ] **File I/O**: `File::open()`, `read_to_string()`, etc. wrapping C stdio.
-   [ ] **String Utils**: `split`, `trim`, `replace`.
-   [ ] **Env & Args**: Accessing command line args and environment variables.
-   [ ] **Result<T, E> Adoption**: Ensure all stdlib functions return `Result`, not exceptions.
-   [ ] **Runtime `ast` Library**: `import ast` to parse, walk, and analyze `.desi` source files. Uses the same parser the compiler uses internally. Enables user-built linters, code generators, and documentation tools.

### 3. Performance Tooling
-   [ ] **Performance Advisor**: Compile-time static analysis for algorithmic anti-patterns (`DPR` diagnostic codes). Three levels: relaxed, default, strict. Configurable via `[diagnostics] perf = "default"` in `desi.mod`.
-   [ ] **`@perf` Decorator**: Runtime benchmarking with iteration count, timing statistics, and automatic stripping in release builds.
-   [ ] **`desic perf` Subcommand**: Run all `@perf`-decorated functions and display timing results.

### 4. Security & Build Audit
-   [ ] **`[permissions]` in desi.mod**: `allow`/`deny` lists for API sensitivity tiers (Safe, System, Network, Privileged). Build fails if denied APIs are used by any dependency.
-   [ ] **Audit Report**: `desic build` with `audit = true` prints a full report of what APIs each dependency uses before building.
-   [ ] **Transitive Scanning**: The compiler scans the entire import graph, including third-party code, for sensitive API usage.

### 5. Safety & Polish
-   [ ] **Panic Handler**: Defined behavior for `unwrap()` failure (print stack trace + exit).
-   [ ] **Error Messages**: Polished, Rust-style compiler error messages (already mostly done).

---

## 🔮 Future Vision (v0.2.0+ — "The Extensibility Era")

Once v0.1.0 is stable, we unlock the next level of power and extensibility.

### 1. Compile-Time Macro Introspection (Headline Feature)

The most requested feature: **write macro rules in Desi that run during compilation.**

-   **Tree-walking interpreter** embedded in the Go compiler for a Desi subset.
-   **Full AST access**: Walk function bodies, inspect types, emit diagnostics.
-   **Sandboxed execution**: No file I/O, no networking, instruction/memory limits.
-   **User diagnostic codes**: `USR` prefix for user-defined warnings/errors.
-   **Same macro protocol**: Hooks into existing `OnCheck` lifecycle — no new APIs.

```desi
import ast
import diag

@macro(target="func")
class sql_safety:
    def on_check(self, func_ast: ast.FuncDecl):
        for call in func_ast.find_calls("db.raw_query"):
            if call.args[0] is ast.FString:
                diag.warn("USR0010", call.span,
                          "f-string in raw SQL — possible injection")
```

This is explicitly deferred from v0.1.0 to protect launch stability. See the full design: [compile_time_macros.md](todo/compile_time_macros.md).

### 2. Hybrid Memory Management (The "Holy Grail")
-   **Escape Analysis**: Compiler automatically decides: "Does this object live forever? → Heap (RC). Does it die here? → Arena."
-   **No Lifetimes**: We want to avoid user-facing validity annotations (`<'a>`) if possible.
-   **Automatic arena promotion**: The compiler detects function-scoped allocations and auto-promotes them to arena allocation without explicit `using arena:` blocks.

### 3. Advanced Concurrency
-   **Async/Await v2**: True non-blocking I/O with LLVM coroutines.
-   **Thread Safety v2**: Compiler-enforced `Send` / `Sync` traits to prevent data races at compile time.

### 4. Advanced Type System
-   **Traits / Interfaces**: Defining shared behavior (like Rust traits or Go interfaces) instead of just inheritance.
-   **Generic Inheritance Fix**: Properly solving the `class D(B<T>)` type parameter propagation.
-   **Variance**: Covariance/contravariance for generic type parameters.

### 5. Hot Reload (Development Mode)
-   **Native + Dynamic Linking**: Recompile only changed modules to `.so`/`.dylib`, hot-swap via `dlopen`/`dlsym`.
-   **State preservation**: `write_state()`/`read_state()` hooks for seamless state transfer across reloads.
-   **Zero production overhead**: Release builds are fully static — dynamic linking is dev-mode only.

---

## 📝 Developer Notes (Keeping Sane)

-   **Why `Result<T,E>`?** Exceptions are hidden control flow. Values are explicit.
-   **Why Wrappers?** Writing a stdlib from scratch is fun but dangerous. Using C's stdlib gives us battle-tested reliability for v1.
-   **Stability First:** If a feature is "flaky", cut it from v0.1.0. We want users to trust the compiler.
-   **Compile-time macros are v0.2.0.** We considered including them in v0.1.0 but the interpreter adds significant stability risk. The runtime `ast` library and declarative macros cover 80%+ of use cases in the meantime.

Let's build this! 🚀
