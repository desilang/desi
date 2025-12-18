# Tuples in Desi

## Overview

Tuples are fixed-size, immutable, heterogeneous collections. Unlike lists, tuples can hold elements of different types and their structure is known at compile time.

```desi
let point: tuple[int, int] = (10, 20)
let person: tuple[str, int, bool] = ("Alice", 30, true)
```

## Design Philosophy

Desi's tuple implementation combines the best of Python and Rust:

| Aspect | Python | Rust | **Desi** |
|--------|--------|------|----------|
| Syntax | `(1, 2)` | `(1, 2)` | `(1, 2)` |
| Indexing | `t[0]` runtime | `t.0` only | **Both** `t.0` and `t[0]` |
| Type safety | Runtime | Compile-time | **Compile-time** |
| Named tuples | `namedtuple` | Use struct | **First-class `(x: int)`** |
| Iteration | Always | Never | **Homogeneous only** |

---

## Basic Tuple Usage

### Creating Tuples

```desi
# Type annotation
let point: tuple[int, int] = (10, 20)

# Type inference
let mixed = (42, "hello", true)  # tuple[int, str, bool]

# Empty tuple - NOT recommended, use unit type instead
# Single element - NOT supported, use the value directly
```

> **Design Decision: No Single-Element Tuples**
> 
> Python requires `(42,)` for single-element tuples. Desi doesn't support this.
> If you have one value, just use that value directly. Tuples are for grouping
> multiple values.

### Accessing Elements

#### Dot Syntax (Rust-style)

```desi
let t = (10, "hello", true)

let x: int = t.0     # 10
let y: str = t.1     # "hello"  
let z: bool = t.2    # true
```

#### Bracket Syntax (Python-style)

```desi
let t = (10, "hello", true)

let x: int = t[0]    # 10
let y: str = t[1]    # "hello"
let z: bool = t[2]   # true
```

> **Design Decision: Compile-Time Indexing Only**
> 
> Bracket indexing requires a **compile-time constant**:
> ```desi
> let t = (1, "hello")
> 
> let a = t[0]     # ✅ OK - literal is compile-time constant
> 
> let i = 0
> let b = t[i]     # ❌ Error: tuple index must be compile-time constant
> ```
> 
> **Why?** The type of `t[0]` is `int`, but `t[1]` is `str`. Without knowing
> the index at compile time, we can't determine the return type. This is 
> fundamental to static typing.

### Destructuring

Extract all elements at once:

```desi
let point = (10, 20)
let (x, y) = point

print(x)  # 10
print(y)  # 20
```

Ignore elements with `_`:

```desi
let person = ("Alice", 30, "alice@email.com")
let (name, _, email) = person

print(name)   # Alice
print(email)  # alice@email.com
```

> **Compile-Time Safety**: Destructuring arity must match tuple size:
> ```desi
> let t = (1, 2, 3)
> let (a, b) = t     # ❌ Error: expected 2 elements, tuple has 3
> let (a, b, c) = t  # ✅ OK
> ```

---

## Named Tuples

Named tuples provide field names for better readability:

### Syntax

```desi
# Named tuple type
type Point = (x: int, y: int)

# Creating with named fields
let p: Point = (x=10, y=20)

# Named access
print(p.x)  # 10
print(p.y)  # 20

# Position access still works
print(p.0)  # 10
print(p.1)  # 20
```

### Nominal Typing

Named tuple types are **nominal** - different names mean different types:

```desi
type Point = (x: int, y: int)
type Size = (width: int, height: int)

let p: Point = (x=10, y=20)
let s: Size = (width=10, height=20)

# These are DIFFERENT types even though structure is same
let p2: Point = s   # ❌ Error: cannot assign Size to Point
```

> **Design Decision: Nominal Named Tuples**
> 
> If you named it `Point`, you meant `Point`, not just any `(int, int)`.
> This prevents bugs like mixing up coordinates with dimensions.
> 
> **Real-world example**: The Mars Climate Orbiter crashed because one
> module sent Metric units while another expected Imperial. Nominal typing
> would catch: `type Meters = (value: float)` ≠ `type Feet = (value: float)`

### Converting Between Named Types

Explicit conversion required:

```desi
type Point = (x: int, y: int)
type Size = (width: int, height: int)

def point_from_size(s: Size) -> Point:
    return (x=s.width, y=s.height)

let s = (width=100, height=50)
let p = point_from_size(s)  # Explicit conversion
```

---

## Tuple Iteration

Iteration is allowed **only for homogeneous tuples** (all elements same type):

```desi
# ✅ Homogeneous - iteration allowed
let coords: tuple[int, int, int] = (10, 20, 30)
for c in coords:
    print(c)  # Type is clearly int

# ❌ Heterogeneous - iteration NOT allowed
let mixed: tuple[int, str] = (1, "hello")
for x in mixed:  # Error: cannot iterate heterogeneous tuple
    print(x)     # Hint: use destructuring instead
```

> **Design Decision: Homogeneous Iteration Only**
> 
> When iterating, the loop variable needs a single type. For `tuple[int, str]`,
> `x` would need to be `int | str` (union type), which adds complexity.
> 
> For heterogeneous tuples, use destructuring:
> ```desi
> let (num, text) = (1, "hello")
> print(num)   # int
> print(text)  # str
> ```

---

## Function Returns

Tuples are perfect for returning multiple values:

```desi
def divmod(a: int, b: int) -> tuple[int, int]:
    return (a / b, a % b)

def main():
    let (quotient, remainder) = divmod(17, 5)
    print(quotient)   # 3
    print(remainder)  # 2
```

---

## Advanced Features

### Tuple Spread

Flatten tuples into larger tuples:

```desi
let a = (1, 2)
let b = (3, 4)
let c = (*a, *b)  # (1, 2, 3, 4)
```

### Tuple Concatenation

```desi
let joined = (1, 2) + (3, 4)  # (1, 2, 3, 4)
```

### Rest Pattern

Collect remaining elements:

```desi
let (first, *rest) = (1, 2, 3, 4)
# first: int = 1
# rest: tuple[int, int, int] = (2, 3, 4)
```

### Membership Testing (Homogeneous)

```desi
let nums = (1, 2, 3, 4, 5)
let has_three = 3 in nums  # true
```

### Comparison

Element-wise comparison:

```desi
(1, 2) == (1, 2)   # true
(1, 2) < (1, 3)    # true (second element differs)
(2, 0) > (1, 100)  # true (first element wins)
```

### Tuple Slicing

Extract a portion of a tuple (compile-time indices required):

```desi
let t = (10, 20, 30, 40, 50)

let middle = t[1:4]  # (20, 30, 40) - new tuple
let first_two = t[0:2]  # (10, 20)
let single = t[2:3]  # 30 - returns element, not tuple
```

> **Note**: Single-element slices return the element value directly (no single-element tuples).

### Builtins for Tuples

Homogeneous tuples support `sum`, `min`, and `max`:

```desi
let nums = (10, 20, 5, 30, 15)

let total = sum(nums)  # 80
let smallest = min(nums)  # 5
let largest = max(nums)  # 30
```

---

## Best Practices

### ✅ Do

- Use tuples for fixed-size grouped data
- Use named tuples for clarity: `(x: int, y: int)` vs `tuple[int, int]`
- Use destructuring for multiple return values
- Use `_` to ignore unneeded elements

### ❌ Don't

- Use tuples for homogeneous collections (use `list` instead)
- Nest tuples deeply (hard to read)
- Use anonymous tuples where named would be clearer

---

## Comparison with Python

| Feature | Python | Desi |
|---------|--------|------|
| `(1, 2, 3)` | ✅ | ✅ |
| `(42,)` single | ✅ | ❌ Not supported |
| `t[0]` | ✅ Runtime | ✅ Compile-time only |
| `t[-1]` negative | ✅ | ❌ Not supported |
| `t[1:3]` slicing | ✅ | ✅ Compile-time indices |
| `for x in t` | ✅ Always | ✅ Homogeneous only |
| `namedtuple` | ✅ Clunky | ✅ First-class |
| `t + t2` | ✅ | ✅ |
| `t * 3` | ✅ | ❌ Not supported |
| `sum(t)` | ✅ | ✅ Homogeneous only |
| `min(t)`, `max(t)` | ✅ | ✅ Homogeneous only |

---

## Implementation Notes

For contributors working on tuple implementation:

### Type Representation

Tuples are represented as LLVM struct types at runtime:
```llvm
; tuple[int, str, bool] becomes:
%tuple_int_str_bool = type { i32, ptr, i1 }
```

### Monomorphization

Named tuples generate separate types:
```llvm
; Point and Size are distinct even if same structure
%Point = type { i32, i32 }
%Size = type { i32, i32 }
```

### Key Files

- `ast/expr_tuple.go` - TupleExpr AST node
- `check/expr.go` - Tuple type checking
- `lower/lower_expr.go` - Lowering to HIR
- `types/tuple.go` - Tuple type representation
