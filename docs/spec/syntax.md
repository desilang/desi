# Syntax (Stage-0)

Desi uses **indentation-based blocks** (no braces) and is expression-oriented. Newlines end statements unless an expression clearly continues (inside `()`, `[]`, or after a binary operator).

## Files & modules
```desi
package tool.lexer
import std.{io, fmt}
import tool.common as common
```

## Bindings & assignment

* Immutable by default: `let x = 10`
* Mutable with `mut`: `let mut y = 0`
* `=` is **initialization only**; `:=` is **reassignment**.

```desi
let x = 10
let mut y = 0
y := y + x
```

## Types (annotations where required)

Function parameters and return types are annotated; local `let` may infer.

```desi
def add(a: i32, b: i32) -> i32:
  a + b
```

## Functions & closures

Functions return the value of the last expression if no explicit `return`.
Closures are first-class.

```desi
def apply[T](x: T, f: (T)->T) -> T:
  f(x)

let inc = def(n: i32) -> i32:
  n + 1

let three = apply(2, inc)
```

## Structs & enums (ADTs)

```desi
struct Span:
  start: u32
  end: u32

enum Token:
  Ident(name: str)
  Int(value: i64)
  Plus
  EOF
```

## Pattern matching

`match` is expression-valued. `_` is a catch-all (exhaustiveness checks may be relaxed in Stage-0).

```desi
def show(t: Token) -> void:
  match t:
    Ident(n) => io.println(n)
    Int(v)   => io.println(fmt.int(v))
    _        => io.println("<sym>")
```

## Control flow

```desi
if cond:
  do_this()
elif other:
  do_that()
else:
  otherwise()

while n > 0:
  n := n - 1

for x in range(10):
  io.println(x)
```

## Errors: Result/Option and `?`

```desi
def read_all(path: str) -> Result[str, IoError]:
  let data = fs.read_file(path)?   # on Err, returns early
  data
```

## Immutability & moves

Values move by default; scalars (`i32`, etc.) are `Copy`. Heap objects (e.g., `str`, `Vec`) are ARC-managed. `defer` schedules cleanup at scope exit.

```desi
let mut buf = Vec[u8].with_cap(1024)
defer buf.clear()
```

## Operators & precedence (high → low)

1. call `()`, index `[]`, field `.`
2. unary: `-  !  not`
3. `*  /  %`
4. `+  -`
5. `<  <=  >  >=`
6. `==  !=  is`
7. `and  or`
8. pipeline `|>` (sugar; optional, may be feature-flagged)

```desi
data |> parse() |> validate() |> compute()
```

## Comments & docs

* `#` line comments
* `##` doc comments (associated to the following item)

## Reserved keywords (Stage-0 set)

`package, import, def, let, mut, return, if, elif, else, while, for, in, match, struct, enum, type, as, is, and, or, not, defer, panic`
