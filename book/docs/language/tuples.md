# Tuples

Tuples are fixed-size, immutable collections that can hold different types.

## Creating Tuples

```desi
let point = (10, 20)           # tuple[int, int]
let person = ("Alice", 30)     # tuple[str, int]
```

## Accessing Elements

Use dot syntax with numeric indices:

```desi
let t = (10, "hello", true)

print(t.0)  # 10
print(t.1)  # hello
print(t.2)  # true
```

## Destructuring

Extract all elements at once:

```desi
let point = (10, 20)
let (x, y) = point

print(x)  # 10
print(y)  # 20
```

Ignore elements with `_`:

```desi
let (name, _, email) = ("Alice", 30, "alice@email.com")
# name = "Alice", email = "alice@email.com"
```

### Rest Pattern

Collect remaining elements:

```desi
let t = (1, 2, 3, 4, 5)

let (first, *rest) = t
# first = 1, rest = (2, 3, 4, 5)

let (*init, last) = t
# init = (1, 2, 3, 4), last = 5
```

## Operations

### Concatenation

```desi
let combined = (1, 2) + (3, 4)  # (1, 2, 3, 4)
```

### Spread

```desi
let a = (1, 2)
let b = (3, 4)
let all = (*a, *b)  # (1, 2, 3, 4)
```

### Slicing (Compile-Time)

```desi
let t = (10, 20, 30, 40, 50)

let middle = t[1:4]  # (20, 30, 40)
let single = t[2:3]  # 30 (element, not tuple)
```

### Comparison

```desi
(1, 2) == (1, 2)  # true
(1, 2) < (1, 3)   # true
```

### Membership (Homogeneous)

```desi
let nums = (1, 2, 3, 4, 5)
print(3 in nums)  # true
```

## Builtins

For homogeneous tuples:

```desi
let nums = (10, 20, 5, 30, 15)

print(len(nums))   # 5
print(sum(nums))   # 80
print(min(nums))   # 5
print(max(nums))   # 30
```

## Iteration (Homogeneous)

```desi
let coords = (10, 20, 30)

for c in coords:
    print(c)
```

> **Note**: Iteration only works when all elements have the same type.

## Named Fields

Tuple elements are positional — a tuple type cannot name its fields. This does
**not** parse:

```text
type Point = (x: int, y: int)
```

When the fields deserve names, use a class:

```desi
class Point:
    pub x: int
    pub y: int

    pub def __new__(self, x: int, y: int):
        self.x = x
        self.y = y

let p = Point(10, 20)
print(str(p.x))  # 10
print(str(p.y))  # 20
```

See [Known Limitations](../reference/known-limitations.md).

## Function Returns

Tuples are perfect for returning multiple values:

```desi
def divmod(a: int, b: int) -> tuple[int, int]:
    return (a / b, a % b)

let (quotient, remainder) = divmod(17, 5)
print(quotient)   # 3
print(remainder)  # 2
```

## Key Points

- **Immutable**: Elements cannot be changed after creation
- **Fixed size**: Length is known at compile time
- **Heterogeneous**: Can mix different types
- **No single-element tuples**: Use the value directly
- **Compile-time indices**: All indexing and slicing uses compile-time constants
