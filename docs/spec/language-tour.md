# Desi Language Tour (Stage-1)

A quick, example-driven walkthrough of Desi as implemented in Stage-1.

## Hello

```desi
def main() -> void:
  print("hello, world")
```

`print(...)` accepts `int|str|bool` (and `unknown` while editing). It returns `void`.

---

## Variables (`let`) and mutability

```desi
def demo() -> void:
  let a = 1             # immutable
  let mut b = 2         # mutable

  # parallel binding with per-name annotations
  let x:int, y, z:str = 1, 2, "hi"

  # reassignment
  b := 3                # ok
  a := 4                # error: cannot assign to immutable
```

* Arity must match on both sides.
* Per-name type annotations win over inference.
* `GroupType` syntax exists but is a no-op in Stage-1.

---

## Types & unification (what actually works)

Kinds: `int`, `str`, `bool`, `void`, `future`, `struct`, `enum`.

* `int` and `bool` unify to `int` (truthiness).
* `struct`/`enum` kinds never unify by name; concrete names are checked where relevant.

---

## Assignment (including struct fields)

```desi
def assign_demo() -> void:
  let mut u: User = User{ id: 1, name: "a" }
  let v = User{ id: 2, name: "b" }

  u := v                # ok: same struct type name `User`

  u.id := 10            # ok: field type must match declared field type
  u.name := "x"         # ok
  v.id := 9             # error: `v` is immutable
```

LHS may be identifiers or field chains (`u.a.b := ...`) on a **mutable** struct variable.

---

## Structs & struct literals

```desi
struct User:
  id: int
  name: str

def mk() -> User:
  let u = User{ id: 1, name: "Ada" }
  return u
```

* Field access/assignment is type-checked against the declared field types.
* Struct literal kind is `struct` with the concrete name (`User`).

---

## Enums & `match`

```desi
enum Result:
  Ok: int
  Err: none

def use(r: Result) -> int:
  match r:
    Ok(x):
      return x
    Err:
      return 0
```

Rules:

* Construct variants with `Result.Ok(1)` and `Result.Err()`.
* In `match`:

  * Use `Variant` (payloadless) or `Variant(binder)` (payloadful).
  * Binder only allowed for payloadful variants.
  * `_:` is a wildcard arm.
  * Non-exhaustive matches warn unless you add `_:`.

---

## Control flow

```desi
def flow(a:int) -> int:
  if a:
    return 1
  elif a > 10:
    return 2
  else:
    return 3

def loop(n:int) -> void:
  while n:
    n := n - 1
```

* `if/elif/while` conditions accept `bool` or `int`.
* A tail **expression statement** whose kind matches the function’s return kind suppresses the “missing return” warning.

---

## Functions, visibility, and async/await

### Functions & visibility

```desi
pub def add(a:int, b:int) -> int:
  a + b
```

* `pub` makes a symbol visible to other modules.
* Unqualified cross-module uses are rejected unless brought in via `from ... import`.
* Module-qualified calls (`m.add(...)`) require the callee to be `pub`.

### Async/await

```desi
pub async def read_len(path:str) -> int:
  let s = await fs.read_all(path)  # fs.read_all returns str (sync); this is just for shape
  return str.len(s)
```

Rules:

* `async def f() -> T` declares `f` returning **`future`** with element kind `T`.
* `await`:

  * Only inside `async` functions.
  * Operand must have kind `future`; the result kind is the future’s element kind (best-effort inference from calls).

---

## Imports & aliases

```desi
import util.math as m
from util.strings import join as sjoin, UPPER, Title

def go() -> void:
  print(UPPER)      # public consts only
  m.add(1, 2)       # ok if `add` is pub
  sjoin("a", "b")   # ok (from-import alias)
```

* `import X [as a]` creates a **module alias** (`a` or last segment of `X`).
* `from M import name [as alias]` brings a **public** symbol into scope (const/func/type/struct/enum).
* Using a bare module alias as a value is an error; qualify (`alias.name`).
* Using `alias.Type`/`alias.Struct`/`alias.Enum` as **values** is invalid (not values).

---

## Top-level constants

```desi
pub let VERSION: str = "1.0.0"
let LOCAL_NUM = 42
```

* `pub let mut` is forbidden.
* Public constants must be compile-time literals (`int|str|bool`).

---

## Built-in/standard shims (Stage-1)

* `print(...) -> void`
* `io.println(...) -> void`
* `fs.read_all(path:str) -> str`
* `os.exit(code:int) -> void`
* `mem.free(x) -> void`
* `str.len(s) -> int`
* `str.at(s,i) -> int` (code point)
* `str.from_code(i) -> str`

---

## Expressions refresher

```desi
a + b * c
(a and b) or c
x |> f |> g
-1
not flag
u.field
arr[i]
call(x, y)
```

Operator groups (by precedence, tightest last):

1. `or`
2. `and`
3. `== !=  < <= > >=`
4. `+ -`
5. `* / %`
6. unary `-`, `!`, `not`, `await`
7. postfix call `()`, index `[]`, field `.`
8. pipeline `|>` (left-assoc)

---

## Diagnostics you’ll see

Errors: `DTE0001`, `DTE0002`, `DTE0003`, `DTE0004`, `DTE0005`, `DTE0006`, `DTE0010`, `DTE0011`, `DTE0012`, `DTE1001`, `DTE1002`
Warnings: `DW0001`, `DW0002`, `DW0004`, `DW0006`, `DW0007`
