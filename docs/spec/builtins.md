# Built-ins (Stage-1)

This page lists the language built-ins that are **always in scope** (no import needed), plus the reserved global names we keep for standard shims. The list is intentionally small right now and will grow; we’ll update this doc as features land.

## Status

* **Implemented now**

  * `print(...)` — variadic, side-effecting output; returns `void`.
* **Reserved module roots (not built-ins, require imports in the future)**

  * `io`, `fs`, `os`, `mem`, `str`
    These identifiers are reserved by the compiler (e.g., you can’t use them as import aliases). They point to “std-shim” namespaces we already typecheck for calls like `io.println`, `fs.read_all`, etc., but they are not considered true built-ins.
* **Planned (subject to change)**

  * Python-like: `len(x)`, `type(x)`, `range(...)`, etc.
  * String helpers grouped under `str.*` (e.g., `str.len`, `str.at`, `str.from_code`) — currently available only via the shim surface, not as free built-ins.
  * Selected utilities inspired by Rust/C/C++/Elixir/Java ecosystems, to be vetted per Stage roadmap.

---

## `print(...)`

### Synopsis

```desi
print(arg1, arg2, ..., argN) -> void
```

### Semantics

* Evaluates each argument (left-to-right) and writes a textual representation to the program’s standard output.
* The exact formatting is implementation-defined for now; no formatting placeholders or separators are guaranteed yet.

### Typing rules

* Each argument must be one of:

  * `int`, `str`, `bool`, or **unknown** (during incomplete typing).
* Passing a `void` expression is an error.
* Passing any other kind (e.g., struct/enum/future) is an error.
* The call’s result type is `void`.

### Examples

```desi
print("hello")          # ok
print(1, 2, 3)          # ok
print(true, "!", 42)    # ok

def greet(name: str) -> void:
  print("hi,", name)    # ok
```

### Diagnostics you might see

* *“print arg N is void (no value)”* — you passed an expression that doesn’t produce a value.
* *“print arg N has unsupported kind K”* — e.g., a struct or enum value without a defined textual form.

---

## Reserved namespaces (std shims)

These are **not** built-ins, but we reserve the top-level names to host standard library shims. You shouldn’t shadow them with import aliases, and they may become real modules:

* `io.println(...)` → prints a line; accepts `int/str/bool`; returns `void`.
* `fs.read_all(path: str)` → returns `str`.
* `os.exit(code: int)` → terminates the process; returns `void`.
* `mem.free(x)` → placeholder for memory mgmt; returns `void`.
* `str.len(x) -> int`, `str.at(x, i) -> int`, `str.from_code(i) -> str` → string helpers.

> Note: The above are enforced by the type checker as special-cased *namespace calls*, not free functions in global scope. Treat them as provisional until the standard library is formalized.

---

## Roadmap / Planned built-ins

The following are candidates; exact signatures and behavior may evolve:

* **Python-inspired**

  * `len(x) -> int` — length of strings/collections
  * `type(x) -> str` (or future type object)
  * `range(...)` — iteration helper (pending for loops/iterables spec)

* **String helpers** (likely grouped under `str.*` first, may later mirror as free built-ins)

  * `str.len(s) -> int`, `str.at(s, i) -> int`, `str.from_code(i) -> str`, formatting utilities.

* **Others under consideration**

  * Numeric predicates/conversions (Rust/C-style)
  * I/O conveniences (Elixir/Java-style variants) coordinated with `io.*`

We’ll promote items from “planned” to “implemented” as they ship.

---

## Notes & constraints

* **Shadowing/aliasing:** `print`, `io`, `fs`, `os`, `mem`, `str` are reserved at the top level; you can’t use them as import aliases. This avoids confusing collisions as the std surface grows.
* **Portability:** Built-ins strive to be stable and independent of the host runtime. Any OS-specific behavior (e.g., console encoding) is out of scope for the spec and should be handled by the runtime/CLI.
