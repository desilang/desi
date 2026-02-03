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
| `unwrap_or(default)` | `T` | Returns value or default if Err |
| `expect(msg)` | `T` | Like unwrap but panics with custom message |
| `map(fn)` | `Result<U, E>` | Transforms Ok value: `Ok(x).map(f) -> Ok(f(x))` |

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

### Built-in and_then() Method

The `and_then()` method chains operations that themselves return an Option or Result (also called flatMap or bind):

```desi
def get_even(x: int) -> Option[int]:
    if x % 2 == 0:
        return Option.Some(x)
    return Option.Nothing

let opt: Option[int] = Option.Some(4)
let result = opt.and_then(get_even)  # Some(4)

let opt2: Option[int] = Option.Some(5)
let result2 = opt2.and_then(get_even)  # Nothing (5 is odd)
```

**Key difference from `map()`:**
- `map(f)` wraps the result: `Some(x).map(f) -> Some(f(x))`
- `and_then(f)` uses `f`'s return directly: `Some(x).and_then(f) -> f(x)`

**Use when** your function already returns an Option/Result:
```desi
# Chaining operations that can fail
let result = get_user(id)
    .and_then(get_profile)    # get_profile returns Option
    .and_then(get_settings)   # get_settings returns Option
```

### Built-in or_else() Method

The `or_else()` method provides a fallback when the value is Nothing/Err:

```desi
def fallback_value() -> Option[int]:
    return Option.Some(99)

let opt: Option[int] = Option.Nothing
let result = opt.or_else(fallback_value)  # Some(99)

let some_opt: Option[int] = Option.Some(5)
let result2 = some_opt.or_else(fallback_value)  # Some(5) - original kept
```

**Key difference from `and_then()`:**
- `and_then(f)` calls `f` when Some/Ok
- `or_else(f)` calls `f` when Nothing/Err

**For Result**, the function receives the error:
```desi
def handle_error(e: str) -> Result[int, str]:
    return Result.Ok(0)  # Convert error to default value

let res: Result[int, str] = Result.Err("failed")
let result = res.or_else(handle_error)  # Ok(0)
```

### Built-in unwrap_or_else() Method

The `unwrap_or_else()` method returns the value or computes a default lazily:

```desi
def compute_default() -> int:
    return 99

let opt: Option[int] = Option.Nothing
let val = opt.unwrap_or_else(compute_default)  # 99 (fn called)

let some_opt: Option[int] = Option.Some(5)
let val2 = some_opt.unwrap_or_else(compute_default)  # 5 (fn NOT called)
```

**Comparison with `unwrap_or`:**
- `unwrap_or(default)` - default is always evaluated
- `unwrap_or_else(fn)` - fn only called when needed (lazy)

**Use lazy evaluation for expensive defaults:**
```desi
# Expensive - always computed even if unused
let val = opt.unwrap_or(expensive_computation())

# Lazy - only computed if needed
let val = opt.unwrap_or_else(expensive_computation)
```

### The `?` Operator (Error Propagation)

The `?` operator provides concise error propagation. When applied to a Result or Option:
- **On Ok/Some**: extracts and returns the value
- **On Err/Nothing**: early returns from the function with the error

```desi
def double_or_propagate(res: Result[int, str]) -> Result[int, str]:
    let n = res?  # If Err, return early with that Err
    return Result.Ok(n * 2)

def get_doubled(opt: Option[int]) -> Option[int]:
    let n = opt?  # If Nothing, return early with Nothing
    return Option.Some(n * 2)
```

> [!IMPORTANT]
> The enclosing function must return a compatible Result/Option type. Using `?` in a function that returns `int` will cause a compile error.

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

## Runtime Limits

Desi enforces runtime safety limits to prevent runaway programs:

### Recursion Depth

Maximum recursion depth is **1000** by default. Exceeding this triggers a panic:

```desi
def infinite_recurse(n: int) -> int:
    infinite_recurse(n + 1)  # Panics after 1000 calls

def main() -> int:
    infinite_recurse(0)  # Desi panic: maximum recursion depth exceeded (1000)
    0
```

This protects against stack overflow from:

- Unbounded recursion
- Deeply nested function calls
- Accidental infinite loops via recursion

### Configuring the Limit

Use `set_recursion_limit(n)` to adjust the limit at runtime:

```desi
def deep_recurse(n: int) -> int:
    if n <= 0:
        return 0
    return deep_recurse(n - 1)

def main() -> int:
    set_recursion_limit(10000)  # Increase limit
    print(deep_recurse(5000))   # Works now
    0
```

### Tail Call Optimization

Desi automatically optimizes **self-recursive tail calls**:

```desi
# Tail recursive - optimized, no stack growth
def factorial(n: int, acc: int) -> int:
    if n <= 1:
        return acc
    return factorial(n - 1, n * acc)  # Tail call

def main() -> int:
    print(factorial(100000, 1))  # Works without stack overflow
    0
```

**Tail position**: A call is in tail position when its result is immediately returned without further processing.

## See Also

- [Types](types.md) - Option and Result type definitions
- [Control Flow](control-flow.md) - Pattern matching with match
- [Functions](functions.md) - Return types and early returns
