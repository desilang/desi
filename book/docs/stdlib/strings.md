# Strings Module

The `strings` module provides 44 utility functions for string manipulation — from basic case conversion to case convention converters, text processing, and character checks.

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

### Case Convention Converters

Convert between naming conventions — no more rewriting these in every project.

| Function | Description | Example |
|----------|-------------|---------|
| `camel_case(s)` | camelCase | `"hello_world"` → `"helloWorld"` |
| `pascal_case(s)` | PascalCase | `"hello_world"` → `"HelloWorld"` |
| `snake_case(s)` | snake_case | `"helloWorld"` → `"hello_world"` |
| `kebab_case(s)` | kebab-case | `"helloWorld"` → `"hello-world"` |
| `screaming_snake(s)` | SCREAMING_SNAKE | `"helloWorld"` → `"HELLO_WORLD"` |

```desi
import strings

# Convert between API response keys and code conventions
let api_key = "firstName"
print(strings.snake_case(api_key))      # "first_name"
print(strings.kebab_case(api_key))      # "first-name"
print(strings.screaming_snake(api_key)) # "FIRST_NAME"

# Reverse: code to API
let code_name = "user_email_address"
print(strings.camel_case(code_name))    # "userEmailAddress"
print(strings.pascal_case(code_name))   # "UserEmailAddress"
```

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
| `is_numeric(s)` | `bool` | True if string is a valid number (int or float) |

### Text Utilities

Functions that save time in every project:

| Function | Returns | Description |
|----------|---------|-------------|
| `truncate(s, max, suffix)` | `str` | Truncate with suffix: `"Hello World"` → `"Hello..."` |
| `slugify(s)` | `str` | URL-friendly slug: `"Hello World!"` → `"hello-world"` |
| `word_wrap(s, width)` | `str` | Wrap text at word boundaries |
| `dedent(s)` | `str` | Remove common leading whitespace (like Python's `textwrap.dedent`) |
| `abbreviate(s, max)` | `str` | Abbreviate at word boundary: `"Hello World"` → `"Hello..."` |

### Split & Join

| Function | Returns | Description |
|----------|---------|-------------|
| `split(s, sep)` | `list[str]` | Split string by separator |
| `join(sep, parts)` | `str` | Join list of strings with separator |
| `splitlines(s)` | `list[str]` | Split by newlines (handles `\n`, `\r\n`, `\r`) |

## Usage Examples

### Text Processing

```desi
import strings

def main() -> int:
    let name = "  John Doe  "
    let clean = strings.trim(name)
    print(strings.upper(clean))       # "JOHN DOE"
    print(strings.swapcase("Hello"))  # "hELLO"

    # Python 3.9+ style
    print(strings.removeprefix("test_file", "test_"))  # "file"
    print(strings.removesuffix("app.desi", ".desi"))   # "app"

    # Padding and formatting
    print(strings.pad_left("42", 5, "0"))    # "00042"
    print(strings.center("hi", 9, "*"))      # "***hi****"

    # Desi extras
    print(strings.slugify("Hello World!"))   # "hello-world"
    print(strings.word_wrap("A very long sentence that should be wrapped at forty chars", 40))

    # Numeric validation
    print(strings.is_numeric("3.14"))   # true
    print(strings.is_numeric("abc"))    # false
    print(strings.is_numeric("-42"))    # true
    0
```

### Case Convention Pipeline

```desi
import strings

# API to code conversion pipeline
let api_fields = ["firstName", "lastName", "emailAddress"]
for field in api_fields:
    let db_col = strings.snake_case(field)
    let env_var = strings.screaming_snake(field)
    let css_class = strings.kebab_case(field)
    print(f"{field} → db: {db_col}, env: {env_var}, css: {css_class}")
```

> [!NOTE]
> `split()` and `join()` are also available as **string methods**:
> ```desi
> let parts = "a,b,c".split(",")   # ["a", "b", "c"]
> let joined = ",".join(parts)     # "a,b,c"
> ```

## See Also

- [Path Module](path.md) — Path manipulation
- [OS Module](os.md) — System functions
