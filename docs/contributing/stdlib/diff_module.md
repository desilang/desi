# Diff Module Implementation

Internal documentation for the `diff` standard library module.

## Architecture

```
compiler/lib/diff.desi     → Desi API (unified, lines, equal)
compiler/runtime/diff.c    → C runtime (Myers diff algorithm)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__diff_unified` | `diff.unified` | `(const char*, const char*) → char*` |
| `__diff_lines` | `diff.lines` | `(const char*, const char*) → char*` |
| `__diff_equal` | `diff.equal` | `(const char*, const char*) → int32` (→ bool) |
| `__diff_count_changes` | `diff.count_changes` | `(const char*, const char*) → int32` |

## Implementation Notes

- Uses a line-level Myers diff algorithm for computing minimal edit sequences.
- `unified()` produces output in standard unified diff format.
- `lines()` produces marked output with `+`, `-`, and ` ` prefixes per line.
- `equal()` is a simple `strcmp()` wrapper.
- No handles needed — all functions are pure string transformations.

## Test Coverage

- `examples/510_diff_module.desi`
