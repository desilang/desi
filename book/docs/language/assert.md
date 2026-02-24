# Assert

The `assert` statement is used to check that a condition is true at runtime. If the condition is false, the program immediately stops with an error message showing the line number.

## Basic Usage

```desi
assert 1 + 1 == 2
assert x > 0
assert name == "desi"
```

If the assertion passes, execution continues normally. If it fails, you'll see:

```
assertion failed: line 5: assertion failed
```

## Custom Messages

You can provide a custom message after a comma:

```desi
assert x > 0, "x must be positive"
assert len(items) > 0, "list cannot be empty"
```

On failure:

```
assertion failed: line 3: x must be positive
```

## When to Use Assert

**Use `assert` for:**

- Checking preconditions in functions
- Verifying invariants during development
- Writing test assertions

```desi
def divide(a: int, b: int) -> int:
    assert b != 0, "division by zero"
    return a / b

@test
def test_math():
    assert 2 + 2 == 4, "basic addition"
    assert 10 / 2 == 5, "basic division"
```

## Behavior

- **Passes:** Execution continues to the next statement
- **Fails:** Prints `assertion failed: line N: message` to stderr and exits with code 1
- Works with any boolean expression: comparisons, logical operators, function calls that return bool
