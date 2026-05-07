# Testing Module Implementation

Internal documentation for the `testing` standard library module.

## Architecture

```
compiler/lib/testing.desi     → Desi API (assertion functions)
compiler/runtime/testing.c    → C runtime (comparisons, counters)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__testing_assert_eq` | `testing.assert_eq` | `(int64, int64, char*) → int32` |
| `__testing_assert_ne` | `testing.assert_ne` | `(int64, int64, char*) → int32` |
| `__testing_assert_gt` | `testing.assert_gt` | `(int64, int64, char*) → int32` |
| `__testing_assert_lt` | `testing.assert_lt` | `(int64, int64, char*) → int32` |
| `__testing_assert_gte` | `testing.assert_gte` | `(int64, int64, char*) → int32` |
| `__testing_assert_lte` | `testing.assert_lte` | `(int64, int64, char*) → int32` |
| `__testing_assert_true` | `testing.assert_true` | `(int32, char*) → int32` |
| `__testing_assert_false` | `testing.assert_false` | `(int32, char*) → int32` |
| `__testing_assert_eq_float` | `testing.assert_eq_float` | `(double, double, char*) → int32` |
| `__testing_assert_close` | `testing.assert_close` | `(double, double, double, char*) → int32` |
| `__testing_assert_eq_str` | `testing.assert_eq_str` | `(char*, char*, char*) → int32` |
| `__testing_assert_ne_str` | `testing.assert_ne_str` | `(char*, char*, char*) → int32` |
| `__testing_assert_contains` | `testing.assert_contains` | `(char*, char*, char*) → int32` |
| `__testing_assert_not_contains` | `testing.assert_not_contains` | `(char*, char*, char*) → int32` |
| `__testing_assert_starts_with` | `testing.assert_starts_with` | `(char*, char*, char*) → int32` |
| `__testing_assert_ends_with` | `testing.assert_ends_with` | `(char*, char*, char*) → int32` |
| `__testing_fail` | `testing.fail` | `(char*) → int32` |
| `__testing_skip` | `testing.skip` | `(char*) → int32` |
| `__testing_summary` | `testing.summary` | `() → int32` |
| `__testing_reset` | `testing.reset` | `() → int32` |
| `__testing_pass_count` | `testing.pass_count` | `() → int32` |
| `__testing_fail_count` | `testing.fail_count` | `() → int32` |

## Implementation Notes

- The C runtime maintains global `pass_count` and `fail_count` counters.
- Each assertion function prints `[PASS]` or `[FAIL]` to stderr and increments the appropriate counter.
- `summary()` prints the final pass/fail counts.
- `reset()` zeroes both counters — useful for multiple test phases.
- This module does **not** use handles — all state is global.

## Test Coverage

- `examples/503_testing_module.desi` — integer, float, bool, string assertions + summary
