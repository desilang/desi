# Builtins

Desi provides built-in functions that are always available without any import. These are part of the language prelude.

## Type Conversions

```desi
str(42)        # "42"
str(3.14)      # "3.14"
str(true)      # "true"
int("42")      # 42
float("3.14")  # 3.14
bool(1)        # true
bool(0)        # false
```

## Character & Encoding

### chr(n)

Convert a Unicode codepoint (integer) to a single-character string:

```desi
print(chr(65))     # A
print(chr(9731))   # ☃ (snowman)
print(chr(128640)) # 🚀
```

### ord(s)

Convert the first character of a string to its Unicode codepoint:

```desi
print(ord("A"))    # 65
print(ord("☃"))    # 9731
```

Roundtrip: `ord(chr(n)) == n` and `chr(ord(s)) == s` (for single-char strings).

## Number Formatting

### hex(n)

Format an integer as a hexadecimal string with `0x` prefix:

```desi
print(hex(255))    # 0xff
print(hex(0))      # 0x0
print(hex(-255))   # -0xff
```

### oct(n)

Format an integer as an octal string with `0o` prefix:

```desi
print(oct(8))      # 0o10
print(oct(0))      # 0o0
print(oct(-8))     # -0o10
```

### bin(n)

Format an integer as a binary string with `0b` prefix:

```desi
print(bin(10))     # 0b1010
print(bin(0))      # 0b0
print(bin(-10))    # -0b1010
```

## Math

### abs(n)

Return the absolute value of a number (works with both `int` and `float`):

```desi
print(abs(-5))     # 5
print(abs(-3.14))  # 3.14
print(abs(42))     # 42
```

### round(n, digits=0)

Round a float to the specified number of decimal places:

```desi
print(round(3.14159))      # 3.0
print(round(3.14159, 2))   # 3.14
print(round(3.14159, 4))   # 3.1416
```

### pow(base, exp)

Integer exponentiation:

```desi
print(pow(2, 10))  # 1024
print(pow(5, 0))   # 1
print(pow(3, 3))   # 27
```

## Collections

| Function | Description |
|----------|-------------|
| `len(x)` | Length of string, list, dict, set, or tuple |
| `sum(items)` | Sum of a list of numbers |
| `min(items)` | Minimum value in a list |
| `max(items)` | Maximum value in a list |
| `any(items)` | `true` if any element is truthy |
| `all(items)` | `true` if all elements are truthy |
| `sorted(items)` | New sorted list (ascending) |
| `sorted(items, reverse=true)` | New sorted list (descending) |
| `reversed(items)` | Iterator in reverse order |
| `zip(a, b)` | Pair elements from two iterables |
| `enumerate(items)` | Iterator of `(index, element)` pairs |

## Functional

| Function | Description |
|----------|-------------|
| `map(fn, items)` | Apply function to each element |
| `filter(fn, items)` | Keep elements where function returns `true` |
| `reduce(fn, items, initial)` | Left fold |
| `foldl(fn, items, initial)` | Left fold (alias for `reduce`) |
| `foldr(fn, items, initial)` | Right fold |

## I/O

| Function | Description |
|----------|-------------|
| `print(...)` | Print values separated by space, ending with newline |
| `input(prompt)` | Read a line from stdin |
| `open(path, mode)` | Open a file |

## Debugging

### dbg(expr)

Print `[file:line] expr = value` and return the value unchanged:

```desi
let x = dbg(2 + 3)  # prints: [main.desi:1] 2 + 3 = 5
# x is now 5
```

### type_of(value)

Get runtime type information:

```desi
let t = type_of(42)
print(t)  # Type[int]
```

### todo()

Panic with "not implemented" — useful as a placeholder during development:

```desi
def process_data(data: str) -> int:
    todo("implement data processing")
```

Inspired by Rust's `todo!()` macro. Calling `todo()` will print a message and exit with code 1.

## Identity & Hashing

| Function | Description |
|----------|-------------|
| `hash(value)` | Hash of a value (FNV-1a of pointer) |
| `id(value)` | Unique identity integer for an object |

## Testing

| Function | Description |
|----------|-------------|
| `assert(condition)` | Panic if condition is false |
| `assert(condition, msg)` | Panic with message if false |
| `assert_eq(expected, actual)` | Panic if values differ |
| `assert_ne(a, b)` | Panic if values are equal |

## See Also

- [Math Module](../stdlib/math.md) — Additional math functions (`sqrt`, `sin`, `cos`, etc.)
- [Strings Module](../stdlib/strings.md) — String manipulation functions
