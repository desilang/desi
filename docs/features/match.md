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

Match on enum variants with payload extraction. Both qualified and unqualified patterns are supported:

```desi
enum Status:
    Pending: none
    Running: int
    Complete: str

let s = Status.Running(42)

# Using qualified patterns (EnumName.Variant)
let msg1 = match s:
    Status.Pending: "waiting"
    Status.Running(id): f"running job {id}"
    Status.Complete(m): f"done: {m}"

# Using unqualified patterns (just Variant)
let msg2 = match s:
    Pending: "waiting"
    Running(id): f"running job {id}"
    Complete(m): f"done: {m}"

# Mixing qualified and unqualified
let msg3 = match s:
    Status.Pending: "qualified pending"
    Running(id): f"unqualified running {id}"
    Complete(m): "unqualified complete"
```

## Generic Enums (Option, Result)

```desi
# Option with unqualified patterns
let opt: Option[int] = Option.Some(42)
let r = match opt:
    Some(v): f"got {v}"
    Nothing: "nothing"

# Result with unqualified patterns
let res: Result[int, str] = Result.Ok(100)
let r = match res:
    Ok(v): f"success: {v}"
    Err(e): f"error: {e}"
```

## Class Types in Generics

Option and Result work correctly with class types:

```desi
class Point:
    pub mut x: int
    pub mut y: int

let opt: Option[Point] = Option.Some(myPoint)
match opt:
    Some(pt): print(pt.x, pt.y)
    Nothing: print("no point")
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

Wildcards can also ignore bindings:

```desi
match status:
    Running(_): "running something"  # Ignores the payload
    _: "other"
```

## Exhaustiveness Checking

The type checker warns if not all enum variants are covered:

```desi
enum Color:
    Red: none
    Green: none
    Blue: none

let c = Color.Red
match c:
    Red: "red"
    Green: "green"
    # Warning: missing Blue variant
```

Add `_` wildcard to suppress the warning if intentional.

## Match as Expression

Match is an expression that returns a value:

```desi
let result: str = match x:
    0: "zero"
    _: "other"

# Use in assignments
let label = match enabled:
    true: "enabled"
    false: "disabled"
```

## Guard Clauses

Add conditions to patterns with `if`:

```desi
let x = 50
let r = match x:
    n if n > 100: "large"
    n if n > 0: "positive"
    n if n < 0: "negative"
    _: "zero"

# With enum payloads
let opt: Option[int] = Option.Some(42)
let result = match opt:
    Some(v) if v > 50: "big value"
    Some(v): f"small: {v}"
    Nothing: "nothing"
```

The identifier (`n`, `v`) binds to the scrutinee/payload value for use in the guard.

## Implementation Details

### Type Checker (check_match.go)
- Extracts enum type from scrutinee (handles Enum, Generic, and Func return types)
- Validates pattern-variant matching
- Populates `MatchBindings` map for payload extraction
- Performs exhaustiveness checking

### Lowerer (match_lower.go)
- Generates If-Else chain for pattern matching
- Extracts tag from enum memory layout (offset 0)
- Extracts payload for bindings (offset 8)
- Handles both primitive and pointer types correctly

### Memory Layout
```
Enum: [tag: i32 (4 bytes)] [padding (4 bytes)] [payload_ptr: ptr (8 bytes)]
```

## Current Limitations

- **No nested patterns**: `Some(Some(x))` not yet supported
- **No struct patterns**: `Point{x, y}` not yet supported

## Related

- [is Operator](is_operator.md) - Test and extract in one expression
- [Enums](enums.md) - Define enum types
