# Diagnostics (Stage-1)

This document lists the diagnostics the Stage-1 compiler can emit today, with short examples and when they occur. Codes map to entries in `compiler/internal/diag/codes.json`. When an entry is missing from the catalog, the checker falls back to a default ID/title but still uses the same code shown below.

> Tip: Messages include context like “in let”, “in assignment”, or “unqualified cross-module use”, and many errors carry a primary span. The CLI pretty-printer will underline the span and show notes when present.

---

## Errors

### DTE0001 — undefined name

The name is not defined in the current scope.

```desi
def f() -> void:
  print(x)  # x is undefined
```

Emitted at identifier use (with a span when available).

---

### DTE0002 — arity mismatch in grouped binding

Number of names on the left doesn’t match number of values on the right.

```desi
def f() -> void:
  let a, b = 1          # in let
  a, b, c := 1, 2       # in assignment
```

The message includes the context (`let` or `assignment`) and the pair of counts.

---

### DTE0003 — name already defined in this scope

Redeclaration in the same scope (locals and parameter scope).

```desi
def f(x:int) -> void:
  let x = 1  # redeclared in this block
```

Triggered during `scope.define`.

---

### DTE0004 — type mismatch

Expression does not have the expected kind (Stage-1 kinds: `int|str|bool|void|future|struct|enum|unknown`).

Common sites:

* Call arguments:

  ```desi
  pub def add(a:int, b:int) -> int: a + b

  def f() -> void:
    add(1, "x")  # arg 2: expected int, found str
  ```

* Assignment / field assignment:

  ```desi
  struct U: id: int

  def f() -> void:
    let mut u: U = U{ id: 1 }
    u.id := "x"  # expected int, found str
  ```

---

### DTE0005 — return type mismatch

Return expression kind does not match the function’s declared return kind.

```desi
def f() -> int:
  return "nope"
```

Also emitted if `return` lacks an expression in non-`void` functions or supplies one in a `void` function.

---

### DTE0006 — cannot assign to immutable variable

Assignment targets must be declared `mut`.

```desi
def f() -> void:
  let a = 1
  a := 2  # error
```

Also used for field assignments where the base variable is immutable.

---

### DTE0010 — symbol is not public

Cross-module visibility violation.

Cases:

* **From-import** of a non-public symbol (span points at the import item):

  ```desi
  from util.math import internal_add  # not public
  ```

* **Module-qualified call** to a non-public function:

  ```desi
  import util.math as m
  def f() -> void:
    m.internal_add(1, 2)  # not public
  ```

* **Unqualified cross-module use** (calling a function that isn’t local and wasn’t from-imported):

  ```desi
  # util.math has pub def add
  # current file did not `from util.math import add`
  def f() -> void:
    add(1,2)  # unqualified cross-module use; error unless local or from-imported
  ```

A span-carrying variant is used when the syntax node has a span (`ErrNotPublicAt`).

---

### DTE0011 — public constant must be compile-time constant

`pub let` must be a literal (`int|str|bool`) in Stage-1.

```desi
pub let X = 1 + 2   # error (not a bare literal)
pub let Y = "ok"    # ok
```

---

### DTE0012 — public let cannot be mutable

`pub let mut` is forbidden.

```desi
pub let mut X = 1   # error
```

---

### DTE1001 — `await` is only valid inside async functions

`await` is gated to `async def ...` bodies.

```desi
def f() -> void:
  await g()   # error DTE1001
```

---

### DTE1002 — cannot await a non-future value

The awaited expression must have kind `future`.

```desi
async def f() -> int:
  let x = 1
  return await x   # error DTE1002
```

If the awaited expression is a call to an `async` function, the checker extracts the element kind (`RetElem`) and `await` yields that.

---

## Warnings

### DW0001 — unused variable or parameter

Names that are never read (locals or parameters). Prefix with `_` to silence.

```desi
def f(a:int) -> void:
  let b = 1  # both a and b unused -> warnings
```

---

### DW0002 — name shadows an outer binding

A new binding hides an outer scope name.

```desi
def f() -> void:
  let x = 1
  if x:
    let x = 2  # shadows outer x
```

---

### DW0004 — unreachable code: statement after return

A statement appears after a return in the same block.

```desi
def f() -> void:
  return
  print("never")  # unreachable
```

---

### DW0006 — function may fall through without an explicit return

Function declares a non-`void` return but might fall through.

Suppressed when:

* The function returns a `future` (async state machine generators).
* The final statement is an **expression statement** whose kind unifies with the return kind (tail-expr style).

```desi
def add(a:int, b:int) -> int:
  a + b  # ok: tail expr satisfies return

def g() -> int:
  let x = 1
  # no tail expr and no return -> DW0006
```

---

### DW0007 — non-exhaustive match

A `match` over a **known** enum is missing one or more variants and there is no wildcard arm `_:`.

```desi
enum R: Ok: int
       Err: none

def f(r:R) -> int:
  match r:
    Ok(x):
      return x        # warning: missing Err
```

Add the missing variants or a `_:` arm to silence.

> Note: Ensure the `warn.non_exhaustive_match` entry exists in `codes.json` with `DW0007`. If absent, the checker still uses `DW0007` as the fallback ID.

---

## Formatting & spans

* The CLI renderer underlines the primary span and prints secondary notes when provided by the checker (e.g., some visibility/import errors attach the import item span).
* Some messages include structured context in the text (e.g., “in let”, “in assignment”, “unqualified cross-module use”).

---

## Catalog & fallbacks

* The checker looks up `(domain,key)` in the catalog. If missing, it falls back to the hardcoded ID/title shown above, so codes remain stable even if `codes.json` is incomplete.
* Domains used here: `"type"` for errors, `"warn"` for warnings.

---

## Quick reference (by key)

| Domain | Key                     | ID      | Summary                            |
| -----: | ----------------------- | ------- | ---------------------------------- |
|   type | undefined_name          | DTE0001 | name not defined                   |
|   type | arity_mismatch          | DTE0002 | LHS/RHS count mismatch             |
|   type | redeclared_symbol       | DTE0003 | name already defined in this scope |
|   type | type_mismatch           | DTE0004 | expected vs. found kind            |
|   type | wrong_return_kind       | DTE0005 | return kind mismatch               |
|   type | assign_to_immutable     | DTE0006 | cannot assign to immutable         |
|   type | not_public              | DTE0010 | symbol is not public               |
|   type | public_const_not_const  | DTE0011 | pub const must be compile-time     |
|   type | pub_let_mut_forbidden   | DTE0012 | pub let cannot be mutable          |
|   type | await_outside_async     | DTE1001 | `await` only inside async          |
|   type | await_non_future        | DTE1002 | cannot await non-future            |
|   warn | unused_variable         | DW0001  | unused variable/parameter          |
|   warn | shadowed_variable       | DW0002  | shadows outer binding              |
|   warn | unreachable_code        | DW0004  | statement after return             |
|   warn | missing_explicit_return | DW0006  | may fall through                   |
|   warn | non_exhaustive_match    | DW0007  | missing variants (no `_:`)         |

---

### Open catalog items (optional)

Your current `codes.json` already defines everything up to `DTE0012` and most warnings. If you want the CLI to display titles/help for the async/match items, add:

```json
{
  "type": {
    "await_outside_async": {
      "id": "DTE1001",
      "title": "`await` is only valid inside async functions",
      "help": "mark the function as `async` to await values"
    },
    "await_non_future": {
      "id": "DTE1002",
      "title": "cannot await a non-future value",
      "help": "only values of kind `future` can be awaited"
    }
  },
  "warn": {
    "non_exhaustive_match": {
      "id": "DW0007",
      "title": "non-exhaustive match",
      "help": "handle all variants or add a `_:` wildcard arm"
    }
  }
}
```
