# Match Expressions

Match expressions provide pattern matching in Desi, similar to Rust's `match` or Python's `match`.

## Basic Syntax

```desi
let result = match value:
    pattern1: expression1
    pattern2: expression2
    _: default_expression
```

## Literal Patterns

Match on primitive values directly:

```desi
# Boolean matching
let b = true
let r = match b:
    true: "yes"
    false: "no"

# Integer matching
let x = 5
let r = match x:
    0: "zero"
    1: "one"
    5: "five"
    _: "other"
```

## Enum Patterns

Match on enum variants with payload extraction:

```desi
enum Status:
    Pending: none
    Running: int
    Complete: str

let s = Status.Running(42)
let msg = match s:
    Status.Pending: "waiting"
    Status.Running(id): f"running job {id}"
    Status.Complete(m): f"done: {m}"
```

## Generic Enums (Option, Result)

```desi
let opt: Option[int] = Option.Some(42)
let r = match opt:
    Option.Some(v): f"got {v}"
    Option.Nothing: "nothing"

let res: Result[int, str] = Result.Ok(100)
let r = match res:
    Result.Ok(v): f"success: {v}"
    Result.Err(e): f"error: {e}"
```

## Wildcard Pattern

Use `_` to match anything:

```desi
let x = 99
let r = match x:
    0: "zero"
    1: "one"
    _: "other"  # Matches everything else
```

## Exhaustiveness

The type checker warns if not all enum variants are covered:

```desi
enum Color:
    Red: none
    Green: none
    Blue: none

let c = Color.Red
match c:
    Color.Red: "red"
    Color.Green: "green"
    # Warning: missing Blue variant
```

Add `_` wildcard to suppress the warning if intentional.

## Match as Expression

Match is an expression that returns a value:

```desi
let result: str = match x:
    0: "zero"
    _: "other"

# Use in function calls
print(match b: true: "yes", false: "no")
```

## Current Limitations

- **Qualified variant names required**: Must use `Option.Some(v)` not just `Some(v)`
- **Class types in Option/Result**: Known issue with class types in generic enums
- **No guard clauses**: `case x if x > 0` not yet supported
- **No nested patterns**: `Some(Some(x))` not yet supported

## Related

- [is Operator](is_operator.md) - Test and extract in one expression
- [Enums](enums.md) - Define enum types
