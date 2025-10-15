# Desi Language — Policy & Rules

> Pythonic surface • Rust-like safety • Elixir-style async • Performance oriented.
> This document is the **source of truth** for current language decisions that the compiler will enforce.

---

## 1) Names, visibility, and modules

* **Top-level classes are public by default** (exported across files).
  *Rationale:* ergonomic APIs; consistent with our “types are part of public surface” stance.

* **Nested classes are private by default** (not exported), unless explicitly marked `pub`.

```desi
class Outer:
  class Inner:            # private by default
    pass
  pub class Info:         # explicitly exported
    pass
```

* **Structs** and **enums** must be exported with `pub` to be importable. Inside a file they’re visible regardless of `pub`.

* **Symbols imported from another file must be declared `pub` in that file** or the import fails with a visibility error.

* **Keywords, builtin types, operators are reserved** and cannot be redefined or used as import aliases.

---

## 2) Dunders (special methods)

* **All dunder methods MUST be `pub`.** If a dunder is declared without `pub`, it is a **compile-time error**.
  *Rationale:* dunders participate in language-level mechanics (printing, equality, iteration, resource closing, etc.) and need consistent visibility.

* **`__new__` (constructor) is special:**

  * If **not defined**, the class **has an implicit zero-arg constructor**: `ClassName()`; any other arity is a compile-time error. This implicit constructor is **public**.
  * If **defined**, it **must be `pub`**, and it controls `ClassName(...)` arity and behavior.
  * Making `__new__` private is **not allowed** (consistency; avoids “public class but unconstructible” surprises).

```desi
class Note:            # no __new__ → only Note()
  pub title: str

class Account:
  pub id: int
  pub name: str
  pub def __new__(id: int, name: str) -> Account:
    Account{ id: id, name: name }
  pub def __repr__() -> str:
    f"Account({self.id}, {self.name})"
```

* **Factory-only construction (without private `__new__`)**

  Use a **capability-gated constructor**: require a parameter type that only this module can create (e.g., a **private nested token**). External code can’t name or construct the token, so it can’t call the constructor directly.

```desi
class Conn:
  class _CtorCap:   # nested → private by default
    pass

  # Public dunder, but requires a private capability param:
  pub def __new__(cap: Conn._CtorCap, dsn: str) -> Conn:
    Conn{ /* open socket, validate, etc. */ }

# Public factory exposed by the module:
pub def connect(dsn: str) -> Conn:
  return Conn(Conn._CtorCap(), dsn)  # Only this file can make _CtorCap
```

* **Future idea (opt-in): `@factory_only`**

  We may add an opt-in annotation:

```desi
@factory_only
class SecureClient:
  pub def __new__(cap: SecureClient._Cap, cfg: Config) -> SecureClient: ...
```

  **Checker-enforced** behavior (future): disallow `SecureClient(...)` outside the defining module unless a valid capability argument is provided; emit a clear visibility/usage diagnostic with a hint to use the module’s factory function.

* Future dunders like `__iter__`, `__next__`, `__hash__`, `__close__` are reserved and **must** be public when implemented.

---

## 3) Classes vs structs

* **Classes** = behavior + state; **implicit `self`** inside methods (you **do not** write it in the parameter list).
  No static methods in Stage-1/2 (prefer module funcs).

* **Structs** = data-only; **literal-only** construction (`Type{ field: value }`), **not callable** (no `__new__`).

* **Fields visibility**: inside a file all fields are usable; cross-file access requires `pub` field.

---

## 4) Mutability, assignment, and bindings

* **Bindings are immutable by default.** Use `let mut` for mutability.
* Reassign with `:=`. Compound ops: `+= -= *= /= %=`.
* **Destructuring** only in `let` (introducing new names), **not** for reassign:

```desi
let [a, b] = [1, 2]     # ok
# [a, b] = [2, 3]       # error
```

---

## 5) Functions, returns, overloading

* **Implicit return**: the last expression is returned; `return` is allowed for early exits.

* **Multi-return requires type annotation**:

  * `def f(): T1, T2 -> ...` or `def f(): <T1,T2> -> ...`
  * Same-type compression: `(): <T> ->` may return `T,T,...`.

* **Overloading**: same name allowed iff **arity and parameter types differ exactly**.
  No widening or subtyping.

```desi
def show(x: int) -> str:  "i"
def show(x: bool) -> str: "b"
# show(1.0f32) → no match error
```

* **Parameters with defaults** and **named arguments** are supported.

---

## 6) Ownership and borrowing (Stage-3 surface, enforced where parsed)

* Parameter kinds:

  * `T` — **owned (move)**, caller binding becomes moved.
  * `ref T` — **shared borrow (read-only)**.
  * `inout T` — **unique mutable borrow** (caller must pass a **mutable lvalue**).
* Rule: many `ref` **or** one `inout`, not both simultaneously.
* **No `inout` across `await`.** Compile-time error.

---

## 7) Pipelines, lambdas, comprehensions

* Pipeline `|>` feeds the left value(s) into the right call’s **positional** parameters; with multi-returns, it feeds the first **N** results by the RHS arity.
* Lambdas: **no `lambda` keyword**. Use `x => expr` or `(x:int,y:int)=>x+y`.
* **Comprehensions are implemented**:

```desi
[x*x for x in xs if x%2==0]
{k: v for k, v in items if v>0}
#{x for x in xs if x>0}
```

---

## 8) Control flow

* `if/elif/else`, `while`, **single-line** forms allowed:

```desi
if cond: stmt
while c: stmt
```
* `match` works on **anything**; **first match wins**; `_` catch-all is **required** in general (enums may be exhaustively matched in the future to waive `_`):

```desi
match x:
  x < 0: "neg"
  0:     "zero"
  _:     "pos"
```

---

## 9) Async & futures

* `async def` returns `future[T]`; `await` unwraps.
* Helpers like `join`, `with_timeout`, `select` live in std.
* **Select**: first ready wins; include `_` default arm.

---

## 10) Using / RAII / defer

* `defer some_call(...)` runs at scope exit (normal return, `?`, or fall-through).
* `using obj:` schedules `obj.__close__()` at scope exit.
* `using x = EXPR():` binds `x` and schedules `x.__close__()` at scope exit.

---

## 11) Errors: Result/Option, `?`, and `panic`

* `Result[T,E] = Ok(T) | Err(E)`, `Option[T]=Some(T)|None`.
* `expr?` propagates `Err(e)` early; otherwise yields value.
* `panic(msg)` is for **bugs** only (aborts).

---

## 12) Builtins, prelude, and reserved names

* Prelude (always available): `print`, `len`, `bool`, `str`, basic higher-order funcs (`map`, `filter`, `range`), etc.
  **Cannot be shadowed** (compile-time error).

* **Reserved tokens** (keywords, builtin types, operators, punctuators) cannot be redefined or aliased in imports.

---

## 13) Strings, docstrings, comments

* Strings `"..."` with escapes; **f-strings** `f"Hello {name} #{1+2}"`.
* Triple-quoted strings `"""..."""`:

* As first statement in function/class → **docstring** attached to symbol.
* As a standalone statement → **block comment** (ignored by runtime).

---

## 14) Collections & iteration

* Types: `list[T]`, `dict[K,V]`, `set[T]`, `tuple[T1,T2,...]`.
  Sugar in type position: `[T]` ≡ `list[T]`, `{K:V}` ≡ `dict[K,V]`.

* Literals: `[1,2]`, `{"a":1}`, `#{"a","b"}`, `(1,"x")`.

* Membership operator: `"x" in set_or_dict_or_list`.

* **for-in loops** iterate any type supporting the iteration protocol (later via `__iter__`/`__next__`):

```desi
for x in [1,2,3]:
  print(x)
```

---

## 15) Decorators

* Apply to `def`, `async def`, and `class`:

```desi
@trace
def add(x:int,y:int)->int: x+y

@cache(128)
async def slow_add(x:int,y:int=1)->int: ...
```
* Semantics: decorators receive and return callables/classes; metadata is preserved for tooling.

---

## 16) FFI (C ABI)

* **A) Manifest-driven (preferred):** `desi.toml` advertises headers/linking; then just `import` as if native.
* **B) Source-level externs:** `@extern("C", link="...") def name(...)->...` in code.
* Raw pointers via `cptr[T]`, `usize/isize`, `null`, and **`unsafe`** blocks around pointer calls.
* Wrap errors as `Result` in userland.

---

## 17) Formatting & style

* **`desifmt`** is the canonical formatter; idempotent, no options.
* Enforced spacing/indentation; respects docstrings/decorators/import grouping.

---

## 18) Diagnostics

* Rust-style diagnostics with **codes** (e.g., `DTE0004`), clear primary/secondary labels, helpful notes and suggestions.
* `--error-format=json` (later) for IDEs.

---

## 19) Grammar notes

* Indentation-sensitive (`Indent`/`Dedent`/`NL`).
* Function headers support defaults, named args, decorators, async.
* Classes have implicit `self` inside methods.
* `match` requires catch-all `_` (general case).
* Comprehensions are in; lambdas use `=>`.
* Pipelines `|>` have precedence between logicals and equality chain (as specified in EBNF).

---

## 20) Prohibitions & common errors (compiler-enforced)

* **No private dunders** (error).
* **No reassigning immutable bindings** (`let` without `mut`).
* **No destructure-reassign**; only allowed in `let`.
* **Cannot shadow prelude builtins** (`print`, `len`, …).
* **Cannot hold `inout` borrows across `await`.**
* **Structs are not callable** (no `__new__`).
* **Imports require `pub` symbols** on the exporting side.

---

## 21) Handy examples (quick reference)

```desi
# Multi-return + pipeline
def stats(): int, f64 -> 3, 2.5
def add(x:int,y:int)->int: x+y
let s = stats() |> add()    # feeds first two returns (int,int) into add

# Swap with inout
def swap(inout a:int, inout b:int)->none:
  a, b := b, a

# Match with `_` required
def classify(x:int)->str:
  match x:
    x < 0: "neg"
    0:     "zero"
    _:     "pos"

# Async + select
async def race(a:future[int], b:future[str])->str:
  select:
    await a: f"int={a}"
    await b: f"str={b}"
    _:       "none"

# Using/RAII
using file = open("foo.txt"):
  print(len(file.read()))

# Decorators
@trace
def f(x:int)->int: x+1

# Comprehensions
let squares = [x*x for x in range(10) if x%2==0]
```

