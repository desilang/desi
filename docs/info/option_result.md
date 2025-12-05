# Option and Result Types in Desi

Desi implements Rust-style `Option<T>` and `Result<T, E>` types for robust, compile-time safe error handling. These types replace nullable values and exceptions with explicit, type-checked alternatives.

## Overview

| Type | Purpose | Variants |
|------|---------|----------|
| `Option<T>` | Represents an optional value | `Some(value)`, `Nothing` |
| `Result<T, E>` | Represents success or failure | `Ok(value)`, `Err(error)` |

---

## Option<T>

`Option<T>` represents a value that may or may not exist. Use it instead of `null` or sentinel values.

### Creating Option Values

```desi
# Some value exists
let x: Option<int> = Option.Some(42)

# No value (nothing)
let y: Option<str> = Option.Nothing
```

### Checking and Accessing Values

```desi
let x: Option<int> = Option.Some(42)

# Check if value exists
if x.is_some():
    print("Has value")

if x.is_nothing():
    print("No value")

# Safely unwrap (assumes value exists)
let value: int = x.unwrap()  # Returns 42
```

### Pattern Matching

```desi
let x: Option<int> = Option.Some(42)

match x:
    Option.Some(v): print(f"Got: {v}")
    Option.Nothing: print("Nothing here")
```

### Common Use Cases

```desi
# Function that may not return a value
def find_user(id: int) -> Option<str>:
    if id == 1:
        return Option.Some("Alice")
    return Option.Nothing

# Usage
let user = find_user(1)
if user.is_some():
    print(f"Found: {user.unwrap()}")
else:
    print("User not found")
```

---

## Result<T, E>

`Result<T, E>` represents an operation that can succeed with type `T` or fail with error type `E`.

### Creating Result Values

```desi
# Success
let x: Result<int, str> = Result.Ok(100)

# Failure
let y: Result<int, str> = Result.Err("Something went wrong")
```

### Checking and Accessing Values

```desi
let x: Result<int, str> = Result.Ok(100)

# Check success/failure
if x.is_ok():
    print("Success!")
    let value = x.unwrap()  # Returns 100

if x.is_err():
    print("Failed!")
    let error = x.unwrap_err()  # Returns error message
```

### Pattern Matching

```desi
let result: Result<int, str> = Result.Ok(42)

match result:
    Result.Ok(v): print(f"Success: {v}")
    Result.Err(e): print(f"Error: {e}")
```

### Common Use Cases

```desi
# Function that can fail
def divide(a: int, b: int) -> Result<int, str>:
    if b == 0:
        return Result.Err("Division by zero")
    return Result.Ok(a / b)

# Usage
let result = divide(10, 2)
match result:
    Result.Ok(v): print(f"Result: {v}")
    Result.Err(e): print(f"Error: {e}")
```

---

## Method Reference

### Option<T> Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `is_some()` | `bool` | True if contains a value |
| `is_nothing()` | `bool` | True if contains nothing |
| `unwrap()` | `T` | Returns the value (panics if Nothing) |

### Result<T, E> Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `is_ok()` | `bool` | True if operation succeeded |
| `is_err()` | `bool` | True if operation failed |
| `unwrap()` | `T` | Returns success value (panics if Err) |
| `unwrap_err()` | `E` | Returns error value (panics if Ok) |

---

## Best Practices

### ✅ Do

```desi
# Always check before unwrapping
if result.is_ok():
    let value = result.unwrap()

# Use pattern matching for exhaustive handling
match option:
    Option.Some(v): handle_value(v)
    Option.Nothing: handle_missing()

# Return Option for "not found" scenarios
def find(items: list<int>, target: int) -> Option<int>:
    # ...

# Return Result for operations that can fail
def parse_int(s: str) -> Result<int, str>:
    # ...
```

### ❌ Don't

```desi
# DON'T unwrap without checking - may panic!
let value = option.unwrap()  # Dangerous!

# DON'T use Option for errors - use Result instead
def divide(a: int, b: int) -> Option<int>:  # Wrong!
    # Use Result<int, str> to explain WHY it failed

# DON'T ignore the error case
if result.is_ok():
    let v = result.unwrap()
# Missing: what to do on error?
```

---

## When to Use Which

| Scenario | Use |
|----------|-----|
| Value may or may not exist | `Option<T>` |
| Operation can fail with an error | `Result<T, E>` |
| Looking up items that may not exist | `Option<T>` |
| I/O operations, parsing, validation | `Result<T, E>` |
| Default/fallback values | `Option<T>` |
| Need to know *why* something failed | `Result<T, E>` |

---

## Strengths

1. **Compile-Time Safety**: The type system forces you to handle missing values and errors explicitly.

2. **Self-Documenting**: Function signatures clearly indicate failure modes:
   ```desi
   def find_user(id: int) -> Option<User>      # May not find
   def parse_config(path: str) -> Result<Config, str>  # Can fail
   ```

3. **No Null Pointer Exceptions**: Eliminates the billion-dollar mistake.

4. **Pattern Matching**: Elegant handling with `match` expressions.

5. **Performance**: Zero-cost abstraction - no runtime overhead compared to manual checks.

---

## Limitations

1. **No `?` Operator (Yet)**: Unlike Rust, Desi doesn't yet support the `?` operator for ergonomic error propagation:
   ```desi
   # Not yet supported:
   def process() -> Result<int, str>:
       let x = try_something()?  # Would propagate error
   ```

2. **No `unwrap_or()`**: Default value methods are not yet implemented:
   ```desi
   # Not yet supported:
   let value = option.unwrap_or(0)
   ```

3. **No `map()` / `and_then()`**: Functional combinators are not yet available:
   ```desi
   # Not yet supported:
   let result = option.map(lambda x: x * 2)
   ```

4. **Panics on Bad Unwrap**: Calling `unwrap()` on `Nothing` or `unwrap_err()` on `Ok` will panic at runtime.

---

## Implementation Details

### Memory Layout

Both `Option` and `Result` use a tagged union representation:
- **Tag** (4 bytes): Discriminant (0 or 1)
- **Payload** (pointer): Heap-allocated value

### Type Erasure

Generic type parameters are erased at compile time. The constructors use `ptr` for all payload types, enabling polymorphism without code duplication.

---

## Examples

### Complete Example: Safe Division

```desi
def safe_divide(a: int, b: int) -> Result<int, str>:
    if b == 0:
        return Result.Err("Cannot divide by zero")
    return Result.Ok(a / b)

def main() -> int:
    let results = [
        safe_divide(10, 2),
        safe_divide(20, 0),
        safe_divide(15, 3)
    ]
    
    for r in results:
        match r:
            Result.Ok(v): print(f"Result: {v}")
            Result.Err(e): print(f"Error: {e}")
    
    return 0
```

### Complete Example: Optional Configuration

```desi
def get_env(key: str) -> Option<str>:
    # Simulate environment lookup
    if key == "DEBUG":
        return Option.Some("true")
    return Option.Nothing

def main() -> int:
    let debug = get_env("DEBUG")
    
    if debug.is_some():
        print(f"Debug mode: {debug.unwrap()}")
    else:
        print("Debug mode not set, using defaults")
    
    return 0
```

---

## Future Enhancements

The following features are planned for future releases:

- `?` operator for error propagation
- `unwrap_or(default)` and `unwrap_or_else(fn)` methods
- `map()`, `and_then()`, `or_else()` combinators
- `expect(msg)` for custom panic messages
- `ok()` and `err()` to convert between Option and Result
