# Display Trait in Desi - Complete Guide

**Table of Contents:**
1. [Quick Start](#quick-start)
2. [What is the Display Trait?](#what-is-the-display-trait)
3. [Default Display](#default-display)
4. [Custom Display with `impl`](#custom-display-with-impl)
5. [Integration with `print()`](#integration-with-print)
6. [Integration with F-strings](#integration-with-f-strings)
7. [Best Practices](#best-practices)

---

## Quick Start

```desi
# Every struct/class gets a default to_str() automatically
struct Point:
    x: int
    y: int

def main():
    let p: Point = Point(x=10, y=20)
    print(p)           # Uses default Display
    print(p.to_str())  # Explicit call
```

For custom formatting:

```desi
trait Display:
    def to_str() -> str

impl Display for Point:
    def to_str() -> str:
        return f"Point({self.x}, {self.y})"
```

---

## What is the Display Trait?

The Display trait defines how a type is converted to a human-readable string. It's the Desi equivalent of:
- Python's `__str__`
- Rust's `std::fmt::Display`
- Java's `toString()`

### The Trait Definition

```desi
trait Display:
    """Display trait for string representation."""
    def to_str() -> str
```

### Built-in Display Implementations

These types have built-in Display:
- `int` → `"42"`
- `float` → `"3.14"`
- `bool` → `"true"` or `"false"`
- `str` → identity (returns itself)
- All structs/classes → auto-generated default

---

## Default Display

**All structs and classes automatically get a default `to_str()` method.**

```desi
struct User:
    name: str
    age: int

def main():
    let u: User = User(name="Alice", age=30)
    let s: str = u.to_str()  # Works automatically!
```

The default implementation produces a simple string representation.

---

## Custom Display with `impl`

Override the default with a custom implementation:

```desi
trait Display:
    def to_str() -> str

struct Point:
    x: int
    y: int

impl Display for Point:
    def to_str() -> str:
        return f"Point(x={self.x}, y={self.y})"

def main():
    let p: Point = Point(x=5, y=10)
    print(p.to_str())  # Output: Point(x=5, y=10)
```

### Classes with Custom Display

```desi
class Counter:
    pub value: int
    
    pub def increment(self) -> none:
        self.value = self.value + 1
        return

impl Display for Counter:
    def to_str() -> str:
        return f"Counter({self.value})"
```

---

## Integration with `print()`

The `print()` function accepts any type that implements Display:

```desi
struct Point:
    x: int
    y: int

def main():
    let p: Point = Point(x=10, y=20)
    
    # All of these work:
    print(p)              # Uses Display automatically
    print(p.to_str())     # Explicit conversion
    print(42)             # Built-in int Display
    print("hello")        # Built-in str Display
    print(true)           # Built-in bool Display
```

### Multiple Arguments

```desi
def main():
    let name: str = "Alice"
    let age: int = 30
    print(name, age)  # Prints: Alice 30
```

---

## Integration with F-strings

F-strings automatically call `to_str()` on embedded expressions:

```desi
struct Point:
    x: int
    y: int

impl Display for Point:
    def to_str() -> str:
        return f"({self.x}, {self.y})"

def main():
    let p: Point = Point(x=5, y=10)
    let x: int = 42
    
    print(f"Point is: {p}")     # Output: Point is: (5, 10)
    print(f"Value: {x}")        # Output: Value: 42
    print(f"Sum: {1 + 2}")      # Output: Sum: 3
```

---

## Best Practices

### ✅ DO

```desi
# DO: Make to_str() output useful for debugging
impl Display for User:
    def to_str() -> str:
        return f"User(name={self.name}, id={self.id})"
```

```desi
# DO: Keep output concise and readable
impl Display for Point:
    def to_str() -> str:
        return f"({self.x}, {self.y})"
```

### ❌ DON'T

```desi
# DON'T: Make to_str() perform heavy computation
impl Display for Database:
    def to_str() -> str:
        # ❌ This queries the database!
        return f"DB with {self.count_all_rows()} rows"
```

```desi
# DON'T: Include sensitive data
impl Display for User:
    def to_str() -> str:
        # ❌ Password in output!
        return f"User({self.name}, {self.password})"
```

---

## Implementation Status

| Feature | Status |
|---------|--------|
| `trait Display` syntax | ✅ Implemented |
| `impl Display for Type` | ✅ Implemented |
| Default `to_str()` for structs | ✅ Implemented |
| Default `to_str()` for classes | ✅ Implemented |
| `print(Display)` integration | ✅ Implemented |
| F-string `{expr}` uses Display | ✅ Implemented |

---

## Examples

All examples for the Display trait are provided inline throughout this document:

- **Quick Start**: See the opening section for basic usage
- **Default Display**: See "Default Display" section
- **Custom Display**: See "Custom Display with `impl`" section
- **Print Integration**: See "Integration with `print()`" section
- **F-string Integration**: See "Integration with F-strings" section
