# Match Expressions

Match expressions let you test a value against multiple patterns and execute code based on which pattern matches.

## Quick Start

```python
let color = "red"
let result = match color:
    "red": "Stop"
    "green": "Go"
    "yellow": "Slow down"
    _: "Unknown"

print(result)  # Stop
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
let result: Option[int] = Option.Some(42)

match result:
    Option.Some(value): print(f"Got: {value}")
    Option.Nothing: print("No value")
```

## Pattern Bindings

Extract values from enum payloads:

```python
let res: Result[str, str] = Result.Ok("Success!")

let message = match res:
    Result.Ok(msg): f"✓ {msg}"
    Result.Err(err): f"✗ Error: {err}"

print(message)  # ✓ Success!
```

## Wildcard `_`

Matches anything - use as a default case:

```python
let day = 4
let name = match day:
    1: "Monday"
    2: "Tuesday"
    7: "Sunday"
    _: "Some other day"
```

## Match is an Expression

Match returns a value, so you can use it anywhere:

```python
# In variable assignment
let label = match x: 0: "zero", _: "non-zero"

# In function arguments
print(match enabled: true: "ON", false: "OFF")
```

## Tips

- Always handle all cases or use `_` as a catch-all
- Match arms must all return the same type
- Use qualified names for enum variants: `Option.Some`, not just `Some`
