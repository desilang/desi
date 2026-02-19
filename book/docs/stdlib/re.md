# Re Module (Regular Expressions)

The `re` module provides regex matching, searching, and text manipulation using POSIX extended regex syntax.

## Import

```desi
import re
```

## API Reference

| Function | Returns | Description |
|----------|---------|-------------|
| `full_match(pattern, s)` | `bool` | True if entire string matches |
| `is_match(pattern, s)` | `bool` | True if pattern matches anywhere |
| `search(pattern, s)` | `str` | First match (empty if none) |
| `findall(pattern, s)` | `list[str]` | All non-overlapping matches |
| `replace(pattern, s, repl)` | `str` | Replace all matches |
| `split(pattern, s)` | `list[str]` | Split by regex pattern |

## Pattern Syntax

Uses POSIX Extended Regular Expressions (ERE):

| Pattern | Matches |
|---------|---------|
| `[0-9]+` | One or more digits |
| `[a-zA-Z]+` | One or more letters |
| `^hello` | Starts with "hello" |
| `world$` | Ends with "world" |
| `(foo\|bar)` | "foo" or "bar" |
| `.+` | Any character, one or more |

## Usage Example

```desi
import re

def main() -> int:
    # Validation
    if re.full_match("[a-z]+@[a-z]+\\.[a-z]+", "user@example.com"):
        print("Valid email format")

    # Extract numbers
    let nums = re.findall("[0-9]+", "price: $42, qty: 3")
    print(nums)  # ["42", "3"]

    # Clean up text
    let clean = re.replace("[^a-zA-Z0-9 ]", "Hello, World! #2024", "")
    print(clean)  # "Hello World 2024"

    # Split CSV with flexible delimiters
    let parts = re.split("[,;\\t]+", "a,b;c\td")
    print(parts)  # ["a", "b", "c", "d"]
    0
```

## See Also

- [Strings Module](strings.md) — String manipulation
