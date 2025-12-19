# Is and Is Not Operators

The `is` and `is not` operators provide pattern matching and identity comparison in Desi. They're particularly useful for working with `Option` and `Result` types.

## Basic Usage

### Identity Comparison

Compare values for identity (same value or reference):

```desi
let a = 10
let b = 10

if a is b:
    print("a equals b")

if a is not b:
    print("a does not equal b")
```

### Boolean and None Checks

```desi
let flag = true
if flag is true:
    print("Flag is set")

let opt: Option[int] = Option.Nothing()
if opt is none:
    print("Option is empty")
```

## Pattern Matching with Bindings

The real power of `is` comes from extracting values with pattern matching.

### Option Patterns

Extract the value from an `Option`:

```desi
let opt: Option[int] = Option.Some(42)

if opt is Some(val):
    print(f"Got value: {val}")  # val is now available
else:
    print("Option is empty")
```

### Result Patterns

Extract success or error values from a `Result`:

```desi
let result: Result[int, str] = Result.Ok(100)

if result is Ok(value):
    print(f"Success: {value}")

if result is Err(error):
    print(f"Error: {error}")
```

### Complete Example

```desi
def process_data(data: Option[int]) -> str:
    if data is Some(n):
        return f"Processing: {n}"
    else:
        return "No data to process"

def main() -> int:
    let result = process_data(Option.Some(42))
    print(result)  # "Processing: 42"
    return 0
```

## Wildcard Patterns

Use `_` to check a variant without binding:

```desi
let opt: Option[int] = Option.Some(42)

# Check if Some without extracting value
if opt is Some(_):
    print("Has a value")

# Also works with Result
if result is Ok(_):
    print("Operation succeeded")
```

## Negated Patterns

Use `is not` to check the opposite:

```desi
let opt: Option[int] = Option.Nothing()

if opt is not Some(_):
    print("Option is empty")

# Useful for early returns
if data is not Some(val):
    return "No data"
# val is only valid here if pattern matched
```

**Note**: Negated patterns with bindings (`is not Some(x)`) don't create the binding `x` since it would be undefined in the then-branch.

## Important Notes

1. **Negated patterns don't bind**: `is not Some(x)` won't create a binding for `x` since it would be undefined in the else branch.

2. **Bindings are scoped**: Pattern variables are only available inside the `if` block where they're defined.

3. **Type inference**: The extracted value automatically has the correct type based on the `Option[T]` or `Result[T, E]` generic parameters.

## Comparison with Match

For simple checks, `is` is more concise than `match`:

```desi
# Using is (preferred for simple cases)
if opt is Some(val):
    use(val)

# Equivalent match expression
match opt:
    Option.Some(val) => use(val)
    _ => pass
```

Use `match` when you need exhaustive pattern matching or multiple cases.

## Quick Reference

| Pattern | Description |
|---------|-------------|
| `x is y` | Identity comparison |
| `x is not y` | Negated identity |
| `opt is none` | Check for Nothing |
| `opt is Some(v)` | Extract Some value |
| `opt is Some(_)` | Check Some (no binding) |
| `opt is not Some(_)` | Check not Some |
| `res is Ok(v)` | Extract Ok value |
| `res is Err(e)` | Extract Err value |
| `res is Ok(_)` | Check Ok (no binding) |

