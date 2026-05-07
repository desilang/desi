# diff — Text Diffing

Compare two strings and produce diff output.

## Import

```desi
import diff
```

## API Reference

| Function | Description |
|---|---|
| `diff.unified(old, new) -> str` | Unified diff output |
| `diff.lines(old, new) -> str` | Line-by-line diff markers |
| `diff.equal(a, b) -> bool` | Check if two strings are equal |
| `diff.count_changes(old, new) -> int` | Number of changed lines |

## Examples

### Unified Diff

```desi
import diff

def main() -> int:
    let old = "line1\nline2\nline3"
    let new = "line1\nmodified\nline3"

    let result = diff.unified(old, new)
    print(result)
    # --- old
    # +++ new
    # -line2
    # +modified
    0
```

### Count Changes

```desi
import diff

def main() -> int:
    let a = "hello\nworld"
    let b = "hello\nearth"

    print(diff.count_changes(a, b))  # 1
    print(diff.equal(a, b))          # false
    print(diff.equal(a, a))          # true
    0
```
