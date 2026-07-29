# Strings and F-Strings

Desi provides powerful string handling with f-string interpolation.

## String Basics

```desi
let name: str = "Desi"
let greeting: str = "Hello, Desi"
let multiline: str = """
    Multi-line
    string literal
"""

# Concatenation
let full: str = "Hello" + " " + "World"
```

---

## F-Strings (Interpolation)

F-strings let you embed expressions directly in strings:

```desi
let name: str = "Alice"
let age: int = 30

print(f"Hello, {name}!")          # Hello, Alice!
print(f"{name} is {age} years old")  # Alice is 30 years old
print(f"Sum: {1 + 2 + 3}")        # Sum: 6
```

### Variable Interpolation

```desi
let x: int = 42
let msg: str = f"Value is {x}"
```

### Expression Interpolation

```desi
let a: int = 10
let b: int = 20
print(f"Sum: {a + b}")      # Sum: 30
print(f"Product: {a * b}")  # Product: 200
```

### Function Calls

```desi
def greet(name: str) -> str:
    return f"Hello, {name}!"

let who = "Bob"
print(f"Message: {greet(who)}")
```

### Struct Fields

```desi
struct Point:
    x: int
    y: int

def main() -> int:
    let p: Point = Point(x=5, y=10)
    print(f"Point at ({p.x}, {p.y})")
    return 0
```

---

## Custom Display

Define how your types appear in f-strings:

```desi
struct Point:
    x: int
    y: int

impl Display for Point:
    def to_str() -> str:
        return f"({self.x}, {self.y})"

def main() -> int:
    let p: Point = Point(x=5, y=10)
    print(f"Point is {p}")  # Point is (5, 10)
    return 0
```

---

## Format Specifiers

Control how values are formatted using Python-style specifiers:

```desi
let pi = 3.14159265
let count = 42

# Float precision
print(f"Pi: {pi:.2f}")      # Pi: 3.14

# Zero-padded integers
print(f"Count: {count:05d}")  # Count: 00042

# Hex formatting
print(f"Hex: {count:x}")    # Hex: 2a
print(f"HEX: {count:X}")    # HEX: 2A

# Width (right-aligned)
print(f"Width: {count:10d}")  # Width:         42
```

### Supported Specifiers

| Spec | Description | Example |
|------|-------------|---------|
| `.Nf` | Float with N decimal places | `{pi:.2f}` → `3.14` |
| `Nd` | Integer width | `{x:5d}` → `   42` |
| `0Nd` | Zero-padded integer | `{x:05d}` → `00042` |
| `x` | Lowercase hex | `{42:x}` → `2a` |
| `X` | Uppercase hex | `{42:X}` → `2A` |
| `o` | Octal | `{42:o}` → `52` |
| `#x` | Hex with prefix | `{42:#x}` → `0x2a` |

---

## Escaping Braces

Use double braces for literal `{` or `}`:

```desi
print(f"Use {{braces}} like this")
# Output: Use {braces} like this
```

---

## String Methods

```desi
let s: str = "hello world"

# Common operations
let length: int = len(s)        # 11
```

### split()

Split a string by a delimiter, returning a list of strings:

```desi
let csv = "apple,banana,cherry"
let parts = csv.split(",")      # ["apple", "banana", "cherry"]

for item: str in parts:
    print(item)
```

### join()

Join a list of strings with a delimiter:

```desi
let words = ["apple", "banana", "cherry"]
let joined = words.join(", ")   # "apple, banana, cherry"
print(joined)
```

### replace()

Replace all occurrences of a substring:

```desi
let original = "Hello World"
let result = original.replace("World", "Desi")  # "Hello Desi"
print(result)
```

---

## Quick Reference

| Syntax | Description |
|--------|-------------|
| `"text"` | String literal |
| `f"Hello, {name}!"` | F-string interpolation |
| `"{{"` / `"}}"` | Escaped braces |
| `len(s)` | String length |
| `s + t` | Concatenation |
