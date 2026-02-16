# Strings Module

The `strings` module provides utility functions for string manipulation.

## Import

```desi
import strings
```

## API Reference

### Case Conversion

| Function | Description | Example |
|----------|-------------|---------|
| `upper(s)` | All uppercase | `"hello"` → `"HELLO"` |
| `lower(s)` | All lowercase | `"HELLO"` → `"hello"` |
| `capitalize(s)` | First letter upper, rest lower | `"hELLO"` → `"Hello"` |
| `title(s)` | First letter of each word upper | `"hello world"` → `"Hello World"` |

### Search

| Function | Returns | Description |
|----------|---------|-------------|
| `contains(s, sub)` | `bool` | True if `sub` is found in `s` |
| `starts_with(s, prefix)` | `bool` | True if `s` starts with `prefix` |
| `ends_with(s, suffix)` | `bool` | True if `s` ends with `suffix` |
| `index_of(s, sub)` | `int` | Index of first occurrence, or `-1` |
| `count(s, sub)` | `int` | Number of non-overlapping occurrences |

### Transform

| Function | Returns | Description |
|----------|---------|-------------|
| `trim(s)` | `str` | Remove whitespace from both ends |
| `trim_left(s)` | `str` | Remove whitespace from left |
| `trim_right(s)` | `str` | Remove whitespace from right |
| `replace(s, old, new)` | `str` | Replace all occurrences of `old` with `new` |
| `repeat(s, n)` | `str` | Repeat `s` n times |
| `reverse(s)` | `str` | Reverse the string |

### Character Checks

| Function | Returns | Description |
|----------|---------|-------------|
| `is_empty(s)` | `bool` | True if `s` is `""` |
| `is_digit(s)` | `bool` | True if all chars are `0-9` |
| `is_alpha(s)` | `bool` | True if all chars are `a-z`/`A-Z` |
| `is_alnum(s)` | `bool` | True if all chars are alphanumeric |
| `is_space(s)` | `bool` | True if all chars are whitespace |
| `is_upper(s)` | `bool` | True if all alpha chars are uppercase |
| `is_lower(s)` | `bool` | True if all alpha chars are lowercase |

## Usage Examples

### Text Processing

```desi
import strings

def main() -> int:
    let name = "  John Doe  "
    let clean = strings.trim(name)
    let upper = strings.upper(clean)
    print(upper)    # "JOHN DOE"

    # Search
    if strings.contains(clean, "Doe"):
        print("Found Doe")

    # Replace
    let fixed = strings.replace("foo-bar-baz", "-", "_")
    print(fixed)    # "foo_bar_baz"
    0
```

### Input Validation

```desi
import strings

def main() -> int:
    let input = "12345"
    if strings.is_digit(input):
        print("Valid number")

    let code = "ABC"
    if strings.is_upper(code) and strings.is_alpha(code):
        print("Valid code")
    0
```

## See Also

- [Log Module](log.md) — Structured logging
- [OS Module](os.md) — System functions
