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

## assert_eq — Equality Check

`assert_eq` checks that two values are equal, and shows both values on failure:

```desi
assert_eq(42, 42)               # passes
assert_eq("hello", "hello")     # passes
assert_eq(1 + 1, 3)             # fails!
```

On failure, you get a clear diff:

```
assertion failed: line 3: assert_eq failed
  expected: 2
    actual: 3
```

With a custom message:

```desi
assert_eq(result, 100, "score should be 100")
```

**Supported types:** `int`, `str`, `bool`

## assert_ne — Inequality Check

`assert_ne` checks that two values are **not** equal:

```desi
assert_ne(1, 2)           # passes
assert_ne("a", "b")       # passes
assert_ne(42, 42)          # fails!
```

On failure:

```
assertion failed: line 3: assert_ne failed
  values should differ but both are: 42
```

## Testing Pattern

```desi
@test
def test_calculator() -> none:
    assert_eq(add(2, 3), 5, "2 + 3 = 5")
    assert_ne(add(2, 3), 0, "result is not zero")
    assert(is_positive(5), "5 is positive")

def main() -> int:
    test_calculator()
    0
```
