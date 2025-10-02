# Desi Stage-1 Semantics

This document describes the behavior enforced by the Stage-1 checker.

## Names & Scope

* Block-scoped. New blocks (`if/elif/else`, `while`, `match` arms) create child scopes.
* Redeclaration in the same scope is an error (**DTE0003**).
* Shadowing outer scope names is allowed but warns (**DW0002**).
* Unused locals/params warn (**DW0001**) unless the name starts with `_`.

## Types & Kinds

Primitive kinds: `int`, `str`, `bool`, `void`, plus `future` (async placeholder).
User types: **structs** and **enums** (identified by name).
Type unification:

* `int` and `bool` unify to `int` (used for truthiness), others must match.
* `struct`/`enum` kinds don’t unify by name here; correctness is checked by higher-level logic.

## Let Bindings

```
let [mut] a[:T], b[:U] [, ...] [ : GroupType ] = e1, e2 [, ...]
```

* Arity must match (**DTE0002**).
* If a per-name type (e.g. `a:S`) is a struct/enum, it fixes the variable’s kind & stored type name.
* Otherwise, kind/struct/enum name is inferred from RHS when possible.
* Declared builtins (e.g. `: int`) must unify with RHS or error (**DTE0004**).
* `GroupType` is accepted but currently a no-op.
* First write happens at initialization; locals are tracked for unused warnings.

## Assignment

```
x := e
a, b := e1, e2
u.id := e
u.a.b := e
```

* LHS may be identifiers or field chains on a **mutable** struct variable.
* Assigning to an immutable name errors (**DTE0006**).
* Whole-value assignment to struct/enum requires the concrete type to match (same struct/enum name).
* Field assignment checks the field’s declared type against RHS (**DTE0004**).
* Arity must match (**DTE0002**).

## Structs

* `struct` fields carry textual types; field access/assign checks these.
* Struct literal:

  ```
  User{ id: 1, name: "x" }
  ```

  yields `struct` kind with the named type.

## Enums & Match

* Enum variants may be payloadless or payloadful:

  ```
  enum Result:
    Ok: int
    Err: none
  ```
* Construction uses `Enum.Variant(payload?)`.
* `match scrut:` requires an enum (or `unknown` during editing), else error.
* Each arm:

  ```
  Variant[(binder)]: <block>
  _ : <block>                 # wildcard
  ```

  * Unknown variant → error.
  * Duplicate variant → error.
  * Binder allowed only for payloadful variants; otherwise error.
  * Binder gets the declared payload type (struct/enum names preserved).
* Missing variants without `_` → **DW0007** non-exhaustive warning.

## Control Flow

* `if`/`elif`/`while` conditions accept `bool` or `int` (truthy int).
* `return`:

  * In `-> void` functions, `return expr` is an error (**DTE0005**).
  * In non-void functions, `return` with no expr is an error (**DTE0005**).
  * Mismatched kinds → **DTE0005**.
  * A tail expression statement that unifies with the return kind suppresses the “missing return” warning; otherwise **DW0006**.
* Statements after a definite return warn as unreachable (**DW0004**).
* `defer` is only allowed at function top level (Stage-0 rule preserved) and must wrap a call.

## Imports & Visibility

* `import m[ as a]` creates a **module alias** (`a` or last segment). Using a bare alias as a value is an error.
* `from m import f[ as g], CONST[ as C], Type[ as T], Struct[ as S], Enum[ as E]`

  * Items must be **public** in the defining module or **DTE0010** at the import site.
  * Reserved names (keywords/builtins) are rejected as aliases.
* Visibility on use:

  * `a.f(...)`: allowed only if `f` is `pub`; otherwise **DTE0010**.
  * Unqualified `f(...)` from another module is **not** allowed unless it came via a from-import alias; otherwise **DTE0010** (with span if available).
  * Accessing `a.CONST`: allowed only if public; otherwise **DTE0010**.
  * Using `a.Type`/`a.Struct`/`a.Enum` as **values** is an error (they’re not values).

## Top-Level `let` (Constants)

```
[pub] let NAME [: Type] = <expr>
```

* `pub let mut` is forbidden (**DTE0012**).
* Public constants must be compile-time literals (`int`, `str`, `bool`) (**DTE0011**).
* Literal kinds are tracked for intra-module typing.

## Async / Await

* `async def f(...) -> T:` means the function’s *declared return* is `future`, with element kind `T`.
* `await e`:

  * Only valid inside `async` functions (**DTE1001**).
  * `e` must have kind `future`, else **DTE1002**.
  * The kind of `await e` is the future’s element kind (best-effort inference from calls).

## Builtin Shims

* `print(...)` → returns `void`; args must be `int|str|bool` (or `unknown` while editing).
* `io.println(...)` same as above.
* `fs.read_all(path: str)` returns `str`.
* `os.exit(code: int)` returns `void`.
* `mem.free(x)` returns `void`.
* `str.len(s)` returns `int`; `str.at(s, i)` returns `int` (code point); `str.from_code(i)` returns `str`.

## Diagnostics (used by the checker)

* **Errors**: `DTE0001` undefined_name, `DTE0002` arity_mismatch, `DTE0003` redeclared_symbol, `DTE0004` type_mismatch, `DTE0005` wrong_return_kind, `DTE0006` assign_to_immutable, `DTE0010` not_public, `DTE0011` public_const_not_const, `DTE0012` pub_let_mut_forbidden, `DTE1001` await_outside_async, `DTE1002` await_non_future.
* **Warnings**: `DW0001` unused_variable, `DW0002` shadowed_variable, `DW0004` unreachable_code, `DW0006` missing_explicit_return, `DW0007` non_exhaustive_match.
