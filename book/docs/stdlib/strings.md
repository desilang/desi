# Strings Module

The `strings` module provides 35 utility functions for string manipulation, matching and exceeding Python's string methods.

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
| `swapcase(s)` | Swap upper↔lower | `"Hello"` → `"hELLO"` |

### Search

| Function | Returns | Description |
|----------|---------|-------------|
| `contains(s, sub)` | `bool` | True if `sub` is found in `s` |
| `starts_with(s, prefix)` | `bool` | True if `s` starts with `prefix` |
| `ends_with(s, suffix)` | `bool` | True if `s` ends with `suffix` |
| `index_of(s, sub)` | `int` | Index of first occurrence, or `-1` |
| `last_index_of(s, sub)` | `int` | Index of last occurrence, or `-1` |
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
| `removeprefix(s, prefix)` | `str` | Remove prefix if present |
| `removesuffix(s, suffix)` | `str` | Remove suffix if present |
| `pad_left(s, width, fill)` | `str` | Left-pad to width with fill char |
| `pad_right(s, width, fill)` | `str` | Right-pad to width with fill char |
| `center(s, width, fill)` | `str` | Center in field of width |
| `zfill(s, width)` | `str` | Zero-fill, preserving sign: `"-42"` → `"-00042"` |
| `char_at(s, idx)` | `str` | Character at index (supports negative) |

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
| `is_ascii(s)` | `bool` | True if all chars are ASCII (0-127) |
| `is_printable(s)` | `bool` | True if all chars are printable |

### Desi Extras (Beyond Python)

| Function | Returns | Description |
|----------|---------|-------------|
| `truncate(s, max, suffix)` | `str` | `"Hello World"` → `"Hello..."` (max 8 chars) |
| `slugify(s)` | `str` | `"Hello World!"` → `"hello-world"` |

## Usage Examples

### Text Processing

```desi
import strings

def main() -> int:
    let name = "  John Doe  "
    let clean = strings.trim(name)
    print(strings.upper(clean))       # "JOHN DOE"
    print(strings.swapcase("Hello"))  # "hELLO"

    # Search
    print(strings.last_index_of("abcabc", "abc"))  # 3
    print(strings.contains(clean, "Doe"))           # true

    # Python 3.9+ style
    print(strings.removeprefix("test_file", "test_"))  # "file"
    print(strings.removesuffix("app.desi", ".desi"))   # "app"

    # Padding and formatting
    print(strings.pad_left("42", 5, "0"))    # "00042"
    print(strings.center("hi", 9, "*"))      # "***hi****"
    print(strings.zfill("-42", 6))           # "-00042"

    # Desi extras
    print(strings.slugify("Hello World!"))   # "hello-world"
    print(strings.truncate("Long text here", 8, "..."))  # "Long..."
    0
```

> [!NOTE]
> `split()` and `join()` are available as **string methods**, not module functions:
> ```desi
> let parts = "a,b,c".split(",")   # ["a", "b", "c"]
> let joined = ",".join(parts)     # "a,b,c"
> ```

## See Also

- [Path Module](path.md) — Path manipulation
- [OS Module](os.md) — System functions
