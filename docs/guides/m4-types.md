# M4 — Types & Overload Resolution (Phase 1)

This guide summarizes what the M4 checker enforces today. Grammar did not change; this is a **semantic/type pass** on top of the existing parser.

## Concrete types

Basic scalars:
- `int`, `float`, `bool`, `str`, `none`

Constructed:
- `list[T]`, `set[T]`, `dict[K, V]`, `tuple[T1, T2, ...]`, `future[T]` (reserved; minimal checks only)
- function types (internal form) `func(p1, p2, ...) -> ret`
- multi-return (internal form) `multi[T1, T2, ...]`

> The checker prints canonical names like `int`, `list[int]`, `func(int, str) -> bool`, `multi[int, str]`.

## What’s inferred

- **Literals** map to their scalar type (`1 -> int`, `3.14 -> float`, `"x" -> str`, etc.).
- **Identifiers** resolve to bound symbols (locals/params/lets); unknown id → `DTE0001 undefined name`.
- **Binary ops** `+ - * / % **` on `int|float` with **exact** operand types; otherwise → `DTE0008 invalid operand types`.
- **Comparisons** (`< > <= >= == !=`) type to `bool`.
- **Logical** `and/or` require `bool`.
- **Let/Assign/AugAssign**: RHS must be assignable to LHS type; `let x = …` infers from RHS.
- **Lambda**: typed params are supported; untyped params may require annotations (tests stick to typed).

## Calls & exact-match overloading

Overloads are grouped **by name**. The checker selects a unique overload by **arity + exact parameter types**. Examples:

```desi
def f(x:int) -> int:
	return x + 1

def f(x:float) -> float:
	return x * 0.5

def main():
	let a = f(10)      # picks f(int)->int
	let b = f(2.5)     # picks f(float)->float
	let c = f("nope")  # DTE0004 no matching overload
````

Ambiguity (if it could arise) would be reported as `DTE0005 ambiguous overload` (not common in M4 because we match exactly).

Arity mismatches are reported with:

* `DTE0046 wrong number of arguments` (for plain calls), and
* a pipeline-specific message if it came from `|>`.

## Pipeline typing

Pipeline `a |> f(b, c)` is treated as `f(a, b, c)` for checking.

```desi
def add(a:int, b:int) -> int:
	return a + b

def main():
	let x = 1 |> add(2)    # OK → add(1, 2)
	let y = 1 |> add()     # DTE0046: wrong number of arguments
```

If there’s a type error in the inserted position, you’ll see a targeted pipeline diagnostic.

## Comprehensions

Element types propagate to the container:

```desi
def main():
	let xs = [x for x in [1, 2, 3]]       # list[int]
	let ys = {x for x in [1, 2, 3]}       # set[int]
	let d  = {k: v for k in [1,2] for v in [true,false]}  # dict[int, bool]
```

## Multi-return

Returning multiple values forms a `multi[T1, T2, ...]`. Width and element types must match on destructuring and assignment.

```desi
def pair() :
	return (1, "a")

def main():
	let (i, s) = pair()   # OK: (int, str)
	let (a, b, c) = pair()  # DTE0007: arity mismatch
```

## `match` (parse-only surface elsewhere)

For M4, the type of a `match` expression requires **all arms to return the same type**. If not, you’ll get a type mismatch across arms.

```desi
def to_int(b:bool) -> int:
	let x = match b:
		true  -> 1
		false -> 0
	return x
```

## Common diagnostics (checker)

* `DTE0001` **undefined name**
* `DTE0004` **type mismatch** (e.g., assignment/return)
* `DTE0046` **wrong number of arguments**
* `DTE0004`/`DTE0005` **no match / ambiguous overload**
* `DTE0006` **bad pipeline feed** (arity/type)
* `DTE0007` **multi-return arity mismatch**
* `DTE0008` **invalid operand types for operator**

---

### Tips

* Prefer annotating lambda parameters in M4 tests/examples.
* For pipelines, remember it’s literal “insert as first arg.”
* Keep match arms uniform in result type for now.

