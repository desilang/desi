# Desi Language: Vision & Roadmap

**Last Updated:** December 05, 2025  
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

### 3. Safety & Polish
-   [ ] **Panic Handler**: Defined behavior for `unwrap()` failure (print stack trace + exit).
-   [ ] **Error Messages**: Polished, Rust-style compiler error messages (already mostly done).

---

## 🔮 Future Vision (v0.2.0+ - "The Performance Era")

Once v0.1.0 is stable, we unlock the next level of performance and features.

### 1. Hybrid Memory Management (The "Holy Grail")
-   **Arenas**: Switch from `malloc` to Region-based memory management for function scopes.
    -   *Impact:* rapid allocation/deallocation, better cache locality.
-   **Escape Analysis**: Compiler automatically decides: "Does this object live forever? -> Heap (RC). Does it die here? -> Arena."
-   **No Lifetimes**: We want to avoid user-facing validity annotations (`<'a>`) if possible.

### 2. Concurrency
-   **Async/Await**: True non-blocking I/O.
-   **Channels**: Go-style message passing.
-   **Thread Safety**: Compiler-enforced `Send` / `Sync` traits to prevent data races.

### 3. Advanced Type System
-   **Traits / Interfaces**: Defining shared behavior (like Rust traits or Go interfaces) instead of just inheritance.
-   **Generic Inheritance Fix**: Properly solving the `class D(B<T>)` type parameter propagation.

---

## 📝 Developer Notes (Keeping Sane)

-   **Why `Result<T,E>`?** Exceptions are hidden control flow. Values are explicit.
-   **Why Wrappers?** Writing a stdlib from scratch is fun but dangerous. Using C's stdlib gives us battle-tested reliability for v1.
-   **Stability First:** If a feature is "flaky", cut it from v0.1.0. We want users to trust the compiler.

Let's build this! 🚀
