# F-Strings in Desi - Complete Guide

**Table of Contents:**
1. [Quick Start](#quick-start)
2. [Syntax](#syntax)
3. [Expression Interpolation](#expression-interpolation)
4. [Type Safety](#type-safety)
5. [Common Patterns](#common-patterns)
6. [Performance](#performance)

---

## Quick Start

```desi
def main():
    let name: str = "Alice"
    let age: int = 30
    
    print(f"Hello, {name}!")         # Hello, Alice!
    print(f"{name} is {age} years old")  # Alice is 30 years old
    print(f"Sum: {1 + 2 + 3}")       # Sum: 6
```

---

## Syntax

F-strings use the `f"..."` prefix with `{expression}` for interpolation:

```desi
f"literal text {expression} more text"
```

**Rules:**
- Prefix with `f` before the opening quote
- Expressions go inside `{...}`
- Expressions are type-checked at compile time
- Result type is always `str`

### Escaping Braces

Use double braces to include literal `{` or `}`:

```desi
print(f"Use {{braces}} like this")  # Use {braces} like this
```

---

## Expression Interpolation

### Variables

```desi
let x: int = 42
let s: str = f"Value is {x}"
```

### Arithmetic

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

print(f"Message: {greet('Bob')}")
```

### Method Calls

```desi
let items: list[int] = [1, 2, 3]
print(f"Count: {items.len()}")  # Count: 3
```

### Struct Fields

```desi
struct Point:
    x: int
    y: int

def main():
    let p: Point = Point(x=5, y=10)
    print(f"Point at ({p.x}, {p.y})")  # Point at (5, 10)
```

### Custom Types with Display

```desi
struct Point:
    x: int
    y: int

impl Display for Point:
    def to_str() -> str:
        return f"({self.x}, {self.y})"

def main():
    let p: Point = Point(x=5, y=10)
    print(f"Point is {p}")  # Point is (5, 10)
```

---

## Type Safety

All expressions in f-strings are type-checked at compile time:

```desi
let x: int = 42
print(f"Value: {x}")       # ✅ OK - int has Display
print(f"Result: {1 + 2}")  # ✅ OK - expression is int

# Expressions must have Display implementation
# All primitives and user types with to_str() work
```

**Automatic Conversion:**
- `int` → string representation
- `float` → string representation
- `bool` → `"true"` or `"false"`
- `str` → identity
- Custom types → `to_str()` method

---

## Common Patterns

### Debugging Output

```desi
let x: int = 42
let name: str = "test"
print(f"DEBUG: x={x}, name={name}")
```

### Building Messages

```desi
def error_message(code: int, msg: str) -> str:
    return f"Error {code}: {msg}"
```

### Formatting Output

```desi
def format_record(id: int, name: str, active: bool) -> str:
    return f"[{id}] {name} (active: {active})"
```

### Multi-line with Triple Quotes

```desi
let report: str = f"""
Report Summary
--------------
Total items: {total}
Processed: {processed}
Failed: {failed}
"""
```

---

## Performance

**Compile-time optimization:**

F-strings are expanded at compile time to efficient string building:

```desi
# This f-string:
f"Hello, {name}!"

# Compiles to equivalent of:
asprintf("Hello, %s!", name)
```

**Benefits:**
- Single allocation for final string
- No intermediate string objects
- Direct format string to printf-family functions

---

## Examples

All f-string examples are provided inline throughout this document:

- **Quick Start**: See opening section for basic usage
- **Expression Types**: See "Expression Interpolation" section
- **Custom Types**: See "Custom Types with Display" section
