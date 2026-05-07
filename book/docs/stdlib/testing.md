# testing — Assertions & Test Harness

Simple assertion functions for writing tests in Desi.

## Import

```desi
import testing
```

## API Reference

### Integer Assertions

| Function | Description |
|---|---|
| `testing.assert_eq(a, b, msg) -> int` | Assert `a == b` |
| `testing.assert_ne(a, b, msg) -> int` | Assert `a != b` |
| `testing.assert_gt(a, b, msg) -> int` | Assert `a > b` |
| `testing.assert_lt(a, b, msg) -> int` | Assert `a < b` |
| `testing.assert_gte(a, b, msg) -> int` | Assert `a >= b` |
| `testing.assert_lte(a, b, msg) -> int` | Assert `a <= b` |

### Boolean Assertions

| Function | Description |
|---|---|
| `testing.assert_true(val, msg) -> int` | Assert value is `true` |
| `testing.assert_false(val, msg) -> int` | Assert value is `false` |

### Float Assertions

| Function | Description |
|---|---|
| `testing.assert_eq_float(a, b, msg) -> int` | Assert exact float equality |
| `testing.assert_close(a, b, tolerance, msg) -> int` | Assert `|a - b| < tolerance` |

### String Assertions

| Function | Description |
|---|---|
| `testing.assert_eq_str(a, b, msg) -> int` | Assert strings are equal |
| `testing.assert_ne_str(a, b, msg) -> int` | Assert strings are not equal |
| `testing.assert_contains(haystack, needle, msg) -> int` | Assert string contains substring |
| `testing.assert_not_contains(haystack, needle, msg) -> int` | Assert string does not contain substring |
| `testing.assert_starts_with(s, prefix, msg) -> int` | Assert string starts with prefix |
| `testing.assert_ends_with(s, suffix, msg) -> int` | Assert string ends with suffix |

### Test Control

| Function | Description |
|---|---|
| `testing.fail(msg) -> int` | Record an unconditional failure |
| `testing.skip(msg) -> int` | Record a skipped test |
| `testing.summary() -> int` | Print pass/fail summary |
| `testing.reset() -> int` | Reset all counters |
| `testing.pass_count() -> int` | Number of passed assertions |
| `testing.fail_count() -> int` | Number of failed assertions |

## Examples

### Basic Test File

```desi
import testing

def main() -> int:
    # Integer checks
    testing.assert_eq(1 + 1, 2, "basic addition")
    testing.assert_gt(10, 5, "ten is greater than five")

    # String checks
    testing.assert_eq_str("hello", "hello", "string equality")
    testing.assert_contains("hello world", "world", "contains check")

    # Float checks
    testing.assert_close(3.14, 3.14159, 0.01, "pi approximation")

    # Print results
    let passes = testing.pass_count()
    let fails = testing.fail_count()
    print(f"passes: {str(passes)}")
    print(f"fails: {str(fails)}")
    0
```
