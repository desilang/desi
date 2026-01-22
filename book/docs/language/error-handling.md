# Error Handling

Desi uses `Option` and `Result` types for safe error handling, inspired by Rust. This approach catches errors at compile time rather than runtime.

## Philosophy

!!! quote "No Exceptions"
    Desi doesn't have exceptions. Instead, errors are values that must be explicitly handled. This makes error handling visible and enforced by the type system.

## Option Type

`Option<T>` represents a value that may or may not exist:

```desi
# Option<T> has two variants:
# - Some(value) - contains a value of type T
# - Nothing     - no value present
```

### Creating Options

```desi
# Some variant - contains a value
let x: Option<int> = Option.Some(42)

# Nothing variant - no value
let y: Option<str> = Option.Nothing
```

### Pattern Matching

```desi
let value: Option<int> = Option.Some(42)

match value:
    Option.Some(v): print(f"Got value: {v}")
    Option.Nothing: print("No value")
```

### Methods

| Method | Return | Description |
|--------|--------|-------------|
| `is_some()` | `bool` | Returns true if Some |
| `is_nothing()` | `bool` | Returns true if Nothing |
| `unwrap()` | `T` | Returns value or panics if Nothing |
| `unwrap_or(default)` | `T` | Returns value or default if Nothing |
| `expect(msg)` | `T` | Like unwrap but panics with custom message |
| `map(fn)` | `Option<U>` | Transforms value: `Some(x).map(f) -> Some(f(x))` |

```desi
let x: Option<int> = Option.Some(42)

if x.is_some():
    print(f"Unwrapped: {x.unwrap()}")

let y: Option<str> = Option.Nothing
if y.is_nothing():
    print("y is nothing")
```

!!! warning "unwrap() Panics"
    Only use `unwrap()` when you're certain the value is `Some`. In production code, prefer pattern matching.

### Common Use Cases

=== "Nullable Values"
    ```desi
    def find_user(id: int) -> Option<str>:
        if id == 1:
            return Option.Some("Alice")
        return Option.Nothing
    ```

=== "Dict Lookup"
    ```desi
    def get_value(key: str, dict: dict[str, int]) -> Option<int>:
        if key in dict:
            return Option.Some(dict[key])
        return Option.Nothing
    ```

=== "First Element"
    ```desi
    def first<T>(items: list<T>) -> Option<T>:
        if len(items) > 0:
            return Option.Some(items[0])
        return Option.Nothing
    ```

## Result Type

`Result<T, E>` represents either success or failure:

```desi
# Result<T, E> has two variants:
# - Ok(value)  - success with value of type T
# - Err(error) - failure with error of type E
```

### Creating Results

```desi
# Success
let success: Result<int, str> = Result.Ok(100)

# Failure
let failure: Result<int, str> = Result.Err("Something went wrong")
```

### Pattern Matching

```desi
let result: Result<int, str> = Result.Ok(100)

match result:
    Result.Ok(v): print(f"Success: {v}")
    Result.Err(e): print(f"Error: {e}")
```

### Methods

| Method | Return | Description |
|--------|--------|-------------|
| `is_ok()` | `bool` | Returns true if Ok |
| `is_err()` | `bool` | Returns true if Err |
| `unwrap()` | `T` | Returns value or panics |
| `unwrap_err()` | `E` | Returns error or panics |

```desi
let x: Result<int, str> = Result.Ok(100)

if x.is_ok():
    print(f"Unwrapped: {x.unwrap()}")

let y: Result<int, str> = Result.Err("Failed")

if y.is_err():
    print(f"Error: {y.unwrap_err()}")
```

### Common Use Cases

=== "Division"
    ```desi
    def divide(a: int, b: int) -> Result<int, str>:
        if b == 0:
            return Result.Err("Division by zero")
        return Result.Ok(a / b)
    ```

=== "File Operations"
    ```desi
    def read_file(path: str) -> Result<str, str>:
        let file = open(path, "r")
        if file.is_open():
            return Result.Ok(file.read())
        return Result.Err(f"Cannot open {path}")
    ```

=== "Parsing"
    ```desi
    def parse_int(s: str) -> Result<int, str>:
        # Attempt to parse string as integer
        ...
    ```

## The ? Operator

The `?` operator provides concise error propagation:

```desi
# For Result<T, E>:
# - If Ok(v), returns v
# - If Err(e), immediately returns Err(e) from the function

# For Option<T>:
# - If Some(v), returns v
# - If Nothing, immediately returns Nothing from the function
```

### Basic Usage

```desi
def process(input: Result<int, str>) -> Result<int, str>:
    let val = input?  # Unwraps or early returns Err
    return Result.Ok(val * 2)
```

Without `?` operator:

```desi
def process(input: Result<int, str>) -> Result<int, str>:
    match input:
        Result.Ok(v): return Result.Ok(v * 2)
        Result.Err(e): return Result.Err(e)
```

### Chaining

```desi
def complex_operation() -> Result<int, str>:
    let x = step1()?    # If Err, returns immediately
    let y = step2(x)?   # If Err, returns immediately
    let z = step3(y)?   # If Err, returns immediately
    return Result.Ok(z)
```

### With Option

```desi
def find_and_double(items: list<int>, target: int) -> Option<int>:
    let index = find_index(items, target)?  # Returns Nothing if not found
    return Option.Some(items[index] * 2)
```

## Converting Between Types

### Option to Result

```desi
def option_to_result<T>(opt: Option<T>, error: str) -> Result<T, str>:
    match opt:
        Option.Some(v): Result.Ok(v)
        Option.Nothing: Result.Err(error)
```

### Result to Option

```desi
def result_to_option<T, E>(res: Result<T, E>) -> Option<T>:
    match res:
        Result.Ok(v): Option.Some(v)
        Result.Err(_): Option.Nothing
```

## Error Types

### String Errors

Simple approach for quick prototyping:

```desi
def risky_operation() -> Result<int, str>:
    if condition_fails:
        return Result.Err("Something went wrong")
    return Result.Ok(42)
```

### Custom Error Types

For more complex applications:

```desi
enum FileError:
    NotFound: path: str
    PermissionDenied: path: str
    IoError: message: str

def read_config(path: str) -> Result<str, FileError>:
    if not file_exists(path):
        return Result.Err(FileError.NotFound(path=path))
    ...
```

## Best Practices

### ✅ Do

- **Use Option for nullable values**: Clearer than null/nil
- **Use Result for operations that can fail**: File I/O, parsing, network
- **Pattern match exhaustively**: Handle both variants
- **Use ? for error propagation**: Reduces boilerplate
- **Provide meaningful error messages**: Help debugging

### ❌ Don't

- **Don't overuse unwrap()**: It panics on failure
- **Don't ignore errors**: Handle or propagate them
- **Don't catch everything with _**: Be specific about errors
- **Don't use exceptions mentality**: Errors are values, not exceptions

## Common Patterns

### Default Values

```desi
def get_or_default(opt: Option<int>, default: int) -> int:
    match opt:
        Option.Some(v): v
        Option.Nothing: default
```

### Built-in map() Method

Desi provides a built-in `map()` method for Option types:

```desi
def double(x: int) -> int:
    return x * 2

let opt: Option<int> = Option.Some(21)
let mapped = opt.map(double)  # Some(42)

let nothing: Option<int> = Option.Nothing
let mapped2 = nothing.map(double)  # Nothing
```

**Behavior:**
- `Some(x).map(f)` → `Some(f(x))` - transforms the contained value
- `Nothing.map(f)` → `Nothing` - preserves Nothing, function is never called

**Chaining:**
```desi
let result = opt.map(double).map(double)  # Some(84)
```

For reference, this is equivalent to:

```desi
def map_option<T, U>(opt: Option<T>, f: (T) -> U) -> Option<U>:
    match opt:
        Option.Some(v): Option.Some(f(v))
        Option.Nothing: Option.Nothing
```

### Chain Multiple Operations

```desi
def pipeline(input: int) -> Result<int, str>:
    let a = step_a(input)?
    let b = step_b(a)?
    let c = step_c(b)?
    return Result.Ok(c)
```

### Validate Input

```desi
def validate_age(age: int) -> Result<int, str>:
    if age < 0:
        return Result.Err("Age cannot be negative")
    if age > 150:
        return Result.Err("Age seems unrealistic")
    return Result.Ok(age)
```

## Comparison with Other Languages

| Language | Approach |
|----------|----------|
| **Desi** | `Option<T>`, `Result<T,E>`, `?` operator |
| Python | Exceptions, `None` |
| Rust | `Option<T>`, `Result<T,E>`, `?` operator |
| Go | Multiple return values, `error` interface |
| Java | Exceptions, `Optional<T>` |
| Swift | `Optional<T>`, `throws` |

Desi's approach combines Rust's type safety with Python's readable syntax.

## See Also

- [Types](types.md) - Option and Result type definitions
- [Control Flow](control-flow.md) - Pattern matching with match
- [Functions](functions.md) - Return types and early returns
