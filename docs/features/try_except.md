# Error Handling: try/except/finally/raise

Desi provides Python-style error handling via `try`/`except`/`finally`/`raise`. This works alongside Desi's `Result<T, E>` and `Option<T>` types to provide a familiar, ergonomic error handling experience.

## Overview

| Statement | Purpose | Description |
|-----------|---------|-------------|
| `try:` | Guard dangerous code | Wraps a block that may raise exceptions or use `?` |
| `except [Type] [as var]:` | Handle errors | Catches exceptions or `?` error propagation |
| `finally:` | Cleanup | Always runs, whether or not an error occurred |
| `raise` | Signal an error | Raises an exception (v0.1.0: panic; v0.2.0: catchable) |

---

## Basic Usage

### try/except

```desi
try:
    let result = risky_operation()?
    print(result)
except e:
    print("caught: " + e)
```

When the `?` operator encounters an `Err` or `Nothing` inside a `try` block, it redirects to the `except` handler instead of performing an early return.

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

### raise

```desi
# Panics the program with a message (v0.1.0)
raise "something went wrong"
```

**Current behavior (v0.1.0):** `raise` calls `__desi_panic()` which prints the message to stderr and exits the process with code 1.

**Planned behavior (v0.2.0):** `raise` will construct an exception object and use `setjmp`/`longjmp` to unwind to the nearest `except` handler, making it catchable.

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

The expression must evaluate to a `str` (v0.1.0). In v0.2.0, it will accept exception type constructors:

```desi
# v0.1.0 (current)
raise "invalid input"

# v0.2.0 (planned)
raise ValueError("invalid input")
raise ZeroDivisionError("cannot divide by zero")
```

---

## Exception Types (v0.2.0, Planned)

Desi will include a built-in exception hierarchy for runtime errors:

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

### Typed except (v0.2.0)

```desi
try:
    let result = compute()?
except ValueError as e:
    print("bad value: " + str(e))
except ZeroDivisionError as e:
    print("division by zero!")
except Exception as e:
    print("other error: " + str(e))
```

`except Exception` catches all exception subtypes, similar to Python.

---

## Relationship to Result/Option

`try`/`except` complements — not replaces — Desi's `Result<T, E>` and `Option<T>` types:

| Feature | Best For |
|---------|----------|
| `Result<T, E>` | Functions that can fail, explicit error types |
| `Option<T>` | Values that may not exist |
| `?` operator | Ergonomic error propagation |
| `try`/`except` | Catching and recovering from errors in a block |
| `raise` | Signaling an unrecoverable error condition |

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

### Control Flow

The `try`/`except`/`finally` construct is lowered to block-based control flow in the HIR:

1. **Try body** → lowered as a normal block
2. **`?` operator** (inside try) → stores error in a slot, jumps to except block
3. **Except block** → reads the error, binds the variable, runs handler
4. **Finally block** → appended after both success and except paths
5. **Continuation** → normal execution resumes

### Performance

| Operation | Overhead | When |
|-----------|----------|------|
| `try` block (v0.1.0) | **Zero** | Block-based control flow, no runtime cost |
| `try` block (v0.2.0) | **~5-10ns** | `setjmp` saves execution state |
| `?` error path | **~5ns** | Store + branch |
| `raise` (v0.1.0) | N/A | Process exits |
| `raise` (v0.2.0) | **~5-10ns** | `longjmp` unwinds to handler |
| Normal code | **Zero** | No overhead outside `try` blocks |

> **Upgrade path:** v0.1.0 uses block-based control flow (zero cost). v0.2.0 will use `setjmp`/`longjmp` (~5-10ns per try entry). A future version may upgrade to LLVM zero-cost exceptions (`invoke`/`landingpad`) for zero happy-path overhead.

---

## Examples

### Handling File Operations

```desi
def process_file(path: str):
    try:
        let data = read_file(path)?
        let parsed = parse_json(data)?
        print("Processed: " + str(len(parsed)))
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
