# Variables & Types

Learn how to declare variables and use Desi's type system.

---

## Declaring Variables

### Immutable Variables: `let`

Use `let` for variables that won't change:

```python
let name = "Desi"
let age = 25
let pi = 3.14159
```

Trying to reassign a `let` variable is an error:

```python
let x = 10
x = 20  # ❌ Error: cannot assign to immutable variable
```

### Mutable Variables: `var`

Use `var` when you need to modify a variable:

```python
var counter = 0
counter = counter + 1  # ✅ OK
counter = counter + 1  # ✅ OK
print(counter)  # Prints: 2
```

!!! tip "Prefer `let`"
    Use `let` by default. Only use `var` when mutation is needed.

---

## Type Annotations

Desi has strong static typing with inference. You can be explicit:

```python
let name: str = "Desi"
let count: int = 42
let price: float = 19.99
let active: bool = true
```

Or let Desi infer:

```python
let name = "Desi"     # str
let count = 42        # int
let price = 19.99     # float
let active = true     # bool
```

---

## Basic Types

### Numbers

| Type | Description | Example |
|------|-------------|---------|
| `int` | Integer | `42`, `-17`, `0` |
| `float` | Floating point | `3.14`, `-0.5` |
| `i32` | 32-bit signed | `i32(100)` |
| `i64` | 64-bit signed | `i64(1000000)` |

```python
let whole = 42          # int
let decimal = 3.14      # float
let big = i64(9999999)  # i64
```

### Strings

```python
let greeting = "Hello, World!"
let name = "Desi"
let combined = greeting + " " + name  # String concatenation
```

### Booleans

```python
let is_valid = true
let is_empty = false

# Boolean operators
let result = is_valid and not is_empty
```

---

## Type Conversion

Convert between types explicitly:

```python
let x: int = 42
let y: float = float(x)    # int -> float
let z: str = str(x)        # int -> str

let a: float = 3.7
let b: int = int(a)        # float -> int (truncates to 3)
```

---

## Constants

By convention, use UPPER_CASE for constants:

```python
let MAX_SIZE = 1000
let PI = 3.14159265359
let APP_NAME = "MyApp"
```

---

## Multiple Declarations

Declare multiple variables:

```python
let a = 1
let b = 2
let c = 3

# Or compute from previous
let sum = a + b + c
```

---

## Scope

Variables are scoped to their block:

```python
def example():
    let x = 10
    
    if true:
        let y = 20
        print(x)  # ✅ OK - x is in scope
        print(y)  # ✅ OK - y is in scope
    
    print(x)  # ✅ OK
    # print(y)  # ❌ Error - y is out of scope
```

---

## Practical Example

```python
def calculate_area():
    let width: float = 10.5
    let height: float = 20.0
    let area = width * height
    
    print("Width:")
    print(width)
    print("Height:")
    print(height)
    print("Area:")
    print(area)

def main() -> int:
    calculate_area()
    0
```

Output:
```
Width:
10.5
Height:
20.0
Area:
210.0
```

---

## Summary

| Keyword | Mutability | When to Use |
|---------|------------|-------------|
| `let` | Immutable | Default choice |
| `var` | Mutable | When value must change |

---

## Next

Learn about [Functions](functions.md) →
