# Match Expressions

Match expressions let you test a value against multiple patterns and execute code based on which pattern matches.

## Quick Start

```python
let day = 2
let name = match day:
    1: "Monday"
    2: "Tuesday"
    3: "Wednesday"
    _: "Another day"

print(name)  # Tuesday
```

## Boolean Matching

```python
let is_admin = true
let access = match is_admin:
    true: "full access"
    false: "read only"
```

## Number Matching

```python
let score = 85
let grade = match score:
    100: "A+"
    90: "A"
    80: "B"
    _: "C or below"
```

## Enum Matching

The most powerful use of match is with enums:

```python
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

```python
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

```python
let x = 99
let label = match x:
    0: "zero"
    1: "one"
    _: "something else"
```

Ignore bindings you don't need:

```python
match status:
    Running(_): "running (don't care about ID)"
    _: "other"
```

## Works with Classes

Match works with classes inside Option/Result:

```python
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

```python
# In variable assignment
let msg = match success:
    true: "It worked!"
    false: "Try again"

# All arms must return the same type
```

## Guard Clauses

Add conditions to patterns with `if`:

```python
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

## Tips

- Always handle all cases or use `_` as a catch-all
- Match arms must all return the same type
- You can use either `Option.Some(v)` or just `Some(v)` - both work!
- Extract payload values with bindings: `Running(id)` 
- Use `_` inside patterns to ignore values: `Running(_)`
- Add conditions with guards: `n if n > 0: ...`

