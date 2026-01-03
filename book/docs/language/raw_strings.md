# Raw Strings

Raw strings are string literals where escape sequences are **not processed**. The backslash `\` is treated as a literal character.

## Syntax

```python
# Basic raw string
let s = r"hello\nworld"  # Contains literal backslash-n, not a newline

# With hash delimiters (for including quotes)
let quoted = r#"He said "hello""#

# With more hashes (for including the previous pattern)
let nested = r##"Use r#"..."# for raw"##
```

## Use Cases

### Regex Patterns
```python
let phone_pattern = r"\d{3}-\d{3}-\d{4}"
let email_pattern = r"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}"
```

### File Paths (Windows)
```python
let path = r"C:\Users\desi\Documents\project"
```

### JSON Templates
```python
let json = r#"{"name": "value", "count": 42}"#
```

## Escape Sequences

In raw strings, **ALL** escape sequences are literal:

| Escape | Regular String | Raw String |
|--------|---------------|------------|
| `\n` | Newline | Literal `\n` (2 chars) |
| `\t` | Tab | Literal `\t` (2 chars) |
| `\r` | Carriage return | Literal `\r` (2 chars) |
| `\\` | Single backslash | Literal `\\` (2 chars) |
| `\u0041` | Character 'A' | Literal `\u0041` (6 chars) |

## Hash Delimiter Rules

Use hash delimiters when you need to include quotes or the `"#` pattern in your string:

```python
# Need to include a quote? Use one hash:
r#"Say "hi""#

# Need to include "#? Use two hashes:
r##"Pattern: "#..."##

# You can always add more hashes as needed
```

## Edge Cases

```python
# Empty raw string
let empty = r""

# Trailing backslash - works with hash delimiters!
let trailing = r#"path\"#

# Multiline raw strings are allowed
let multi = r"line1
line2"
```

## See Also

- [Examples: 246_raw_strings.desi](/examples/246_raw_strings.desi) - Comprehensive examples
