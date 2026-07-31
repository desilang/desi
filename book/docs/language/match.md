# Match Expressions

Match expressions let you test a value against multiple patterns and execute code based on which pattern matches.

## Quick Start

```desi
let day = 2
let name = match day:
    1: "Monday"
    2: "Tuesday"
    3: "Wednesday"
    _: "Another day"

print(name)  # Tuesday
```

## Boolean Matching

```desi
let is_admin = true
let access = match is_admin:
    true: "full access"
    false: "read only"
```

## Number Matching

```desi
let score = 85
let grade = match score:
    100: "A+"
    90: "A"
    80: "B"
    _: "C or below"
```

## Enum Matching

The most powerful use of match is with enums:

```desi
enum Status:
    Pending: none
    Running: int
    Complete: str

let s = Status.Running(42)
let msg = match s:
    Pending: "waiting..."
    Running(id): f"running job {id}"
    Complete(result): f"done: {result}"

print(msg)  # running job 42
```

## Pattern Bindings

Extract values from enum payloads:

```desi
let result: Option[int] = Option.Some(42)

match result:
    Some(value): print(f"Got: {value}")
    Nothing: print("No value")

# With Result
let res: Result[str, str] = Result.Ok("Success!")
let message = match res:
    Ok(msg): f"✓ {msg}"
    Err(err): f"✗ Error: {err}"
```

## Wildcard `_`

Matches anything - use as a default case:

```desi
let x = 99
let label = match x:
    0: "zero"
    1: "one"
    _: "something else"
```

Ignore bindings you don't need:

```desi
match status:
    Running(_): "running (don't care about ID)"
    _: "other"
```

## Works with Classes

Match works with classes inside Option/Result:

```desi
class Point:
    pub mut x: int
    pub mut y: int

let opt: Option[Point] = Option.Some(myPoint)
match opt:
    Some(pt): print(pt.x, pt.y)
    Nothing: print("no point")
```

## Match is an Expression

Match returns a value, so you can use it anywhere:

```desi
# In variable assignment
let msg = match success:
    true: "It worked!"
    false: "Try again"

# All arms must return the same type
```

## Guard Clauses

Add conditions to patterns with `if`:

```desi
let x = 50
let result = match x:
    n if n > 100: "large"
    n if n > 0: "positive"
    _: "zero or negative"

# Works with enums too
match opt:
    Some(v) if v > 50: "big value"
    Some(v): f"small: {v}"
    Nothing: "none"
```

## Nested Patterns

Match on nested enums:

```desi
let nested: Option[Option[int]] = Option.Some(Option.Some(42))
match nested:
    Some(Some(v)): print(f"value: {v}")
    Some(Nothing): print("inner nothing")
    Nothing: print("outer nothing")
```

## Struct Patterns

Match on struct fields:

```desi
struct Point:
    pub x: int
    pub y: int

let p = Point(x=10, y=20)
match p:
    Point(x, y): print(f"({x}, {y})")

# Also works inside Option/Result
let opt: Option[Point] = Option.Some(Point(x=5, y=15))
match opt:
    Some(Point(x, y)): print(f"point: ({x}, {y})")
    Nothing: print("no point")
```

## Tips

- Always handle all cases or use `_` as a catch-all
- Match arms must all return the same type
- You can use either `Option.Some(v)` or just `Some(v)` - both work!
- Extract payload values with bindings: `Running(id)` 
- Use `_` inside patterns to ignore values: `Running(_)`
- Add conditions with guards: `n if n > 0: ...`
- Match nested enums: `Some(Some(v))`
- Match struct fields: `Point(x, y)`
- Match struct in enum: `Some(Point(x, y))`

