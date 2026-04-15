# Error Handling: try/except/finally/raise

Desi provides Python-style error handling via `try`/`except`/`finally`/`raise`. Exceptions are **catchable** using a `setjmp`/`longjmp` runtime, and work alongside Desi's `Result<T, E>` and `Option<T>` types.

## Overview

| Statement | Purpose | Description |
|-----------|---------|-------------|
| `try:` | Guard dangerous code | Wraps a block that may raise exceptions or use `?` |
| `except [Type] [as var]:` | Handle errors | Catches exceptions by type or as a catch-all |
| `finally:` | Cleanup | Always runs, whether or not an error occurred |
| `raise` | Signal an error | Raises a catchable exception |

---

## Basic Usage

### try/except

```desi
try:
    raise "something went wrong"
except e:
    print("caught: " + e)
```

The `except e:` clause catches **any** exception and binds the error message to `e`.

### raise with a message

```desi
raise "invalid input"
```

Raises a base `Exception` with the given message. If no `except` handler is active, the program exits with an error.

### Typed exceptions

```desi
raise ValueError("expected a number")
raise KeyError("missing config key")
```

Raises a typed exception. The exception type determines which `except` handler catches it.

### Typed except

```desi
try:
    raise ValueError("bad input")
except ValueError as e:
    print("caught ValueError: " + e)
```

Only catches `ValueError` exceptions. If a different exception type is raised, it **propagates** to an outer handler (or terminates the program).

### Catch-all except

```desi
try:
    raise KeyError("missing key")
except e:
    print("caught: " + e)
```

`except e:` (without a type) catches **all** exception types.

### try/except/finally

```desi
try:
    let file = open("data.txt")?
    process(file)
except e:
    print("error: " + e)
finally:
    print("cleanup runs always")
```

The `finally` block always executes, whether or not an exception was caught.

### try/finally (no except)

```desi
try:
    acquire_resource()
finally:
    release_resource()
```

Useful for cleanup-on-exit patterns without catching errors.

---

## Exception Types

Desi includes a built-in exception hierarchy for runtime errors:

```
Exception                  # Base for all catchable exceptions
├── ValueError             # Invalid value (e.g., int("abc"))
├── KeyError               # Dict key not found
├── IndexError             # List index out of range
├── ZeroDivisionError      # Division by zero
├── IOError                # File/network I/O failure
├── RuntimeError           # Generic runtime error
├── OverflowError          # Integer overflow
├── TimeoutError           # Async/network timeout
└── ConnectionError        # Network connection failure
```

Since Desi is **statically typed**, many Python exceptions don't apply:
- `TypeError` → caught at compile time
- `NameError` → caught at compile time (undefined variable)
- `AttributeError` → caught at compile time (missing method/field)

Each type has an integer tag (0–9) used at runtime for fast matching.

---

## How try/except Interacts with `?`

The `?` operator is **context-aware**. Its behavior changes depending on whether it's inside a `try` block:

| Context | `?` on Err/Nothing | Effect |
|---------|---------------------|--------|
| Outside `try` | Early return `Err(e)` | Function returns the error to its caller |
| Inside `try` | Jump to `except` | Error is caught by the handler, no return |

```desi
# Outside try: ? performs early return
def parse_and_double(s: str) -> Result<int, str>:
    let n = parse_int(s)?       # Returns Err("...") on failure
    return Result.Ok(n * 2)

# Inside try: ? redirects to except
def safe_parse(s: str):
    try:
        let n = parse_int(s)?   # Jumps to except on failure
        print(str(n))
    except e:
        print("parse failed: " + e)
```

---

## Syntax Reference

### Full Form

```desi
try:
    <body>
except [ExceptionType] [as <variable>]:
    <handler>
finally:
    <cleanup>
```

All parts are optional except `try:` and at least one of `except` or `finally`:

```desi
# try/except only
try:
    dangerous_operation()
except e:
    handle_error(e)

# try/finally only
try:
    acquire_resource()
finally:
    release_resource()

# try/except/finally
try:
    dangerous_operation()
except e:
    handle_error(e)
finally:
    cleanup()
```

### raise

```desi
raise <expression>
```

The expression can be a string or an exception type constructor:

```desi
# Plain string → base Exception
raise "invalid input"

# Typed exception with message
raise ValueError("invalid input")
raise ZeroDivisionError("cannot divide by zero")
raise KeyError("missing key: " + key)
```

---

## Relationship to Result/Option

`try`/`except` complements — not replaces — Desi's `Result<T, E>` and `Option<T>` types:

| Feature | Best For |
|---------|----------|
| `Result<T, E>` | Functions that can fail, explicit error types |
| `Option<T>` | Values that may not exist |
| `?` operator | Ergonomic error propagation |
| `try`/`except` | Catching and recovering from errors in a block |
| `raise` | Signaling an error when recovery isn't possible locally |

### Recommended pattern

```desi
# Use Result for function signatures
def read_config(path: str) -> Result<Config, str>:
    let file = open(path)?
    let data = parse(file)?
    return Result.Ok(data)

# Use try/except at the call site for recovery
def main():
    try:
        let config = read_config("app.toml")?
        start_app(config)
    except e:
        print("Failed to start: " + e)
        use_defaults()
```

---

## Implementation Details

### Control Flow (setjmp/longjmp)

The `try`/`except`/`finally` construct is lowered to `setjmp`/`longjmp`-based control flow:

1. **Allocate frame** → 256-byte `ExceptionFrame` on stack
2. **`__desi_try_push(frame)`** → push frame onto thread-local handler stack
3. **`setjmp(frame)`** → returns 0 on first call; non-zero when `longjmp` fires
4. **Branch** → `setjmp == 0` → try body; else → except handler
5. **Normal completion** → `__desi_try_pop()`, jump to continuation
6. **Exception** → `__desi_exception_message()` to get the message string
7. **Typed match** → `__desi_exception_matches(tag)` to check exception type
8. **Re-raise** → `__desi_reraise()` if type doesn't match
9. **Finally** → always runs in the continuation block

### Runtime Functions

| Function | Purpose |
|----------|---------|
| `__desi_try_push(frame)` | Push exception handler frame |
| `__desi_try_pop()` | Pop handler frame (normal completion) |
| `__desi_raise(tag, msg, type_name)` | Raise exception via `longjmp` |
| `__desi_exception_message()` | Get current exception's message string |
| `__desi_exception_tag()` | Get current exception's type tag |
| `__desi_exception_matches(tag)` | Check if current exception matches tag |
| `__desi_reraise()` | Re-raise current exception to outer handler |

### Performance

| Operation | Overhead | Notes |
|-----------|----------|-------|
| `try` block entry | **~5-10ns** | `setjmp` saves execution state |
| Normal try exit | **~1ns** | Pop handler frame |
| `?` error path (in try) | **~5ns** | `longjmp` to handler |
| `raise` | **~5-10ns** | `longjmp` unwinds to handler |
| Normal code (outside try) | **Zero** | No overhead |

> **Future optimization:** A later version may upgrade to LLVM zero-cost exceptions (`invoke`/`landingpad`) for zero happy-path overhead.

---

## Examples

### Complete Example: Typed Exception Handling

```desi
def main() -> int:
    # Basic raise + catch
    try:
        raise "oops"
    except e:
        print("caught: " + e)

    # Typed exception
    try:
        raise ValueError("bad input")
    except ValueError as e:
        print("caught ValueError: " + e)

    # Catch-all for unmatched types
    try:
        raise KeyError("missing key")
    except e:
        print("caught base: " + e)

    # finally always runs
    try:
        print("in try body")
    finally:
        print("finally ran")

    print("all tests passed")
    return 0
```

**Output:**
```
caught: oops
caught ValueError: bad input
caught base: missing key
in try body
finally ran
all tests passed
```

### Handling File Operations

```desi
def process_file(path: str):
    try:
        let data = read_file(path)?
        let parsed = parse_json(data)?
        print("Processed: " + str(len(parsed)))
    except IOError as e:
        print("I/O error: " + e)
    except e:
        print("Error processing " + path + ": " + e)
    finally:
        print("Done with " + path)
```

### Nested try/except

```desi
def main():
    try:
        try:
            let inner = dangerous_inner()?
            print(str(inner))
        except e:
            print("inner error: " + e)
            raise "inner failed, aborting"
    except e:
        print("outer caught: " + e)
```

### Guard with finally

```desi
def with_lock(lock: Mutex):
    lock.acquire()
    try:
        critical_section()
    finally:
        lock.release()  # Always runs
```

---

## Best Practices

### ✅ Do

```desi
# Use typed exceptions for specific error recovery
try:
    let conn = connect(url)?
except ConnectionError as e:
    retry_later()
except e:
    log_error(e)

# Use finally for resource cleanup
try:
    acquire_lock()
    do_work()
finally:
    release_lock()

# Prefer Result<T, E> for function return types
def parse_int(s: str) -> Result<int, str>:
    # ...
```

### ❌ Don't

```desi
# DON'T use raise for normal control flow
# Use Result or Option instead
try:
    if x < 0:
        raise "negative"    # Bad — use Result.Err
except e:
    x = 0

# DON'T catch and ignore exceptions
try:
    dangerous_operation()
except e:
    pass  # Silent failure — always log or handle

# DON'T raise without a message
raise ""  # Unhelpful — use a descriptive message
```

---

## When to Use Which

| Scenario | Use |
|----------|-----|
| Function can fail, caller decides what to do | `Result<T, E>` |
| Value may or may not exist | `Option<T>` |
| Catch and recover from errors in a block | `try`/`except` |
| An error must stop the current operation | `raise` |
| Cleanup that must always run | `try`/`finally` |
| I/O, parsing, network operations | `try`/`except` + `Result<T, E>` |
