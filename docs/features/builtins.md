# Built-in Functions

Desi provides several Python-like built-in functions for common operations on collections and values.

---

## Collection Functions

### len()

Returns the number of elements in a collection.

```desi
let items: list[int] = [1, 2, 3, 4, 5]
let size: int = len(items)  # 5

let text: str = "hello"
let length: int = len(text)  # 5

let scores: dict[str, int] = {"alice": 100, "bob": 85}
let count: int = len(scores)  # 2
```

**Accepted Types:**
- `list[T]` → `int`
- `str` → `int`
- `dict[K, V]` → `int`
- `set[T]` → `int`
- Any class with `__len__` method

---

### sum()

Returns the sum of all elements in a list.

```desi
let numbers: list[int] = [1, 2, 3, 4, 5]
let total: int = sum(numbers)  # 15
```

**Accepted Types:**
- `list[int]` → `int`
- `list[float]` → `float`

---

### min() / max()

Returns the minimum or maximum value in a list.

```desi
let numbers: list[int] = [3, 1, 4, 1, 5, 9]
let smallest: int = min(numbers)  # 1
let largest: int = max(numbers)   # 9
```

**Accepted Types:**
- `list[int]` → `int`
- `list[float]` → `float`

---

### sorted()

Returns a **new** list with elements sorted in ascending order. The original list is unchanged.
Supports an optional `reverse` parameter.

```desi
let numbers: list[int] = [3, 1, 4, 1, 5, 9, 2, 6]

# Standard ascending
let ordered: list[int] = sorted(numbers)
# ordered = [1, 1, 2, 3, 4, 5, 6, 9]

# Descending with named argument
let desc: list[int] = sorted(numbers, reverse=true)
# desc = [9, 6, 5, 4, 3, 2, 1, 1]

# Pipeline syntax
let piped: list[int] = numbers |> sorted(reverse=true)
```

**Accepted Types:**
- `list[T]` → `list[T]` (where T is orderable)

> **Note:** For in-place sorting, use the `list.sort()` method (planned).

---

### any()

Returns `true` if **any** element in the list is truthy.

```desi
let flags1: list[bool] = [true, false, false]
let flags2: list[bool] = [false, false]

if any(flags1):
    print("At least one is true")  # ✓ Prints

if any(flags2):
    print("At least one is true")  # Does not print
```

**Semantics:**
- Returns `true` if at least one element is `true`
- Returns `false` for empty lists
- Short-circuits on first `true` value

**Accepted Types:**
- `list[bool]` → `bool`

---

### all()

Returns `true` if **all** elements in the list are truthy.

```desi
let flags1: list[bool] = [true, true, true]
let flags2: list[bool] = [true, false, true]

if all(flags1):
    print("All are true")  # ✓ Prints

if all(flags2):
    print("All are true")  # Does not print
```

**Semantics:**
- Returns `true` if all elements are `true`
- Returns `true` for empty lists (vacuous truth)
- Short-circuits on first `false` value

**Accepted Types:**
- `list[bool]` → `bool`

---

## Transformation Functions

### map()

Applies a function to each element of a list, returning a new list with the results.

```desi
let numbers: list[int] = [1, 2, 3, 4, 5]
let doubled: list[int] = map(lambda x: x * 2, numbers)
# doubled = [2, 4, 6, 8, 10]

# With named function
def square(x: int) -> int:
    return x * x

let squares: list[int] = map(square, numbers)
# squares = [1, 4, 9, 16, 25]
```

**Signature:** `map(func, list[T]) → list[ReturnType(func)]`

**Chaining Syntax:**
```desi
# Dot notation
let doubled: list[int] = numbers.map(square)

# Pipe notation  
let doubled: list[int] = numbers |> map(square)

# Chained
let result: list[int] = numbers.map(double).filter(is_even)
```

---

### filter()

Returns a new list containing only elements that satisfy the predicate.

```desi
let numbers: list[int] = [1, 2, 3, 4, 5, 6]
let evens: list[int] = filter(lambda x: (x % 2) == 0, numbers)
# evens = [2, 4, 6]

# With named function
def is_positive(x: int) -> bool:
    return x > 0

let positives: list[int] = filter(is_positive, [-1, 0, 1, 2])
# positives = [1, 2]
```

**Signature:** `filter(predicate, list[T]) → list[T]`

---

### reduce()

Reduces a list to a single value by applying a function cumulatively from left to right.

```desi
def add(acc: int, x: int) -> int:
    return acc + x

let nums: list[int] = [1, 2, 3, 4, 5]
let total: int = reduce(add, nums, 0)
# Steps: ((((0+1)+2)+3)+4)+5 = 15

def multiply(acc: int, x: int) -> int:
    return acc * x

let product: int = reduce(multiply, nums, 1)
# Steps: ((((1*1)*2)*3)*4)*5 = 120
```

**Signature:** `reduce(func, list[T], initial) → AccT`

**Parameters:**
- `func` - Function `(accumulator, element) → accumulator`
- `list[T]` - List to reduce
- `initial` - Starting value for accumulator

---

### foldl()

Alias for `reduce()`. Left-to-right fold.

```desi
let total: int = foldl(add, nums, 0)  # Same as reduce
```

---

### foldr()

Right-to-left fold. Processes elements from the end of the list.

```desi
let nums: list[int] = [1, 2, 3, 4, 5]
let result: int = foldr(subtract, nums, 0)
# Processes: 5, 4, 3, 2, 1 (right to left)
```

---

### Implementation Details

Both `map()` and `filter()` use **compile-time desugaring** to list comprehensions:

```desi
# What you write:
map(lambda x: x * 2, numbers)
filter(lambda x: x > 0, numbers)

# What the compiler sees:
[(x * 2) for x in numbers]
[x for x in numbers if x > 0]
```

**Why this approach?**

| Approach | Pros | Cons |
|----------|------|------|
| **Desugaring (current)** | Zero overhead, full LLVM optimization, lambda inlining | Eager only |
| Function pointers | Flexible, runtime dispatch | Overhead, no inlining |
| Closure ABI | Full closure support | Complex, boxing overhead |

The desugaring approach was chosen because:
1. **Zero-cost abstraction** — No function pointer indirection
2. **Full optimization** — LLVM sees the entire loop and can vectorize/unroll
3. **Memory predictability** — Eager evaluation means deterministic allocation
4. **Simplicity** — No closure ABI complexity for initial implementation

### Future Improvements

| Feature | Description | Status |
|---------|-------------|--------|
| Lazy iterators | `map(f, xs)` returns iterator, not list | Planned |
| Chaining | `xs.map(f).filter(p)` method syntax | ✅ Done |
| Multi-iterable map | `map(f, xs, ys)` for binary functions | Planned |
| Parallel map | `pmap(f, xs)` for parallel execution | Future |
| Reduce/fold | `reduce(f, xs, init)` builtin | ✅ Done |

> **Note:** For now, use list comprehensions if you need more control:
> ```desi
> [f(x) for x in xs if p(x)]  # Equivalent to filter then map
> ```

---

## Set Iteration

Sets are iterable using the `for` loop. Iteration order is undefined (implementation specific).

```desi
let numbers: set[int] = #{1, 2, 3}
for n: int in numbers:
    print(n)
```

---

## Iteration Functions

### range()

Generates a sequence of integers for iteration.

```desi
# range(stop): 0 to stop-1
for i: int in range(5):
    print(i)  # 0, 1, 2, 3, 4

# range(start, stop): start to stop-1
for i: int in range(2, 6):
    print(i)  # 2, 3, 4, 5

# range(start, stop, step): with step increment
for i: int in range(0, 10, 2):
    print(i)  # 0, 2, 4, 6, 8
```

---

### enumerate()

Returns index-value pairs during iteration.

```desi
let items: list[str] = ["a", "b", "c"]
for i: int, item: str in enumerate(items):
    print(i)     # 0, 1, 2
    print(item)  # "a", "b", "c"
```

---

### reversed()

Iterates over a collection in reverse order.

```desi
let items: list[int] = [1, 2, 3]
for x: int in reversed(items):
    print(x)  # 3, 2, 1
```

---

### zip()

Pairs elements from two collections for parallel iteration.

```desi
let names: list[str] = ["alice", "bob"]
let scores: list[int] = [100, 85]

for name: str, score: int in zip(names, scores):
    print(name)   # "alice", "bob"
    print(score)  # 100, 85
```

> **Note:** Stops at the shorter list's length.

---

## ✅ DO

```desi
# DO: Use any/all for boolean checks
let results: list[bool] = [true, true, false]
if not all(results):
    print("Some checks failed")

# DO: Use sorted to get ordered copy
let nums: list[int] = [3, 1, 2]
let ordered: list[int] = sorted(nums)

# DO: Use enumerate for indexed iteration
for i: int, val: int in enumerate(nums):
    print(i)
```

---

## ❌ DON'T

```desi
# DON'T: Use any/all on non-bool lists
let numbers: list[int] = [1, 2, 3]
any(numbers)  # ❌ Error: any requires list[bool]


# DON'T: Confuse sorted with in-place mutation
let nums: list[int] = [3, 1, 2]
sorted(nums)  # Returns new list, nums unchanged
```

---

## Summary Table

| Function | Input Type | Return Type | Description |
|----------|------------|-------------|-------------|
| `len(x)` | Collection or str | `int` | Number of elements |
| `sum(x)` | `list[int]` or `list[float]` | Element type | Sum of elements |
| `min(x)` | `list[int]` or `list[float]` | Element type | Minimum value |
| `max(x)` | `list[int]` or `list[float]` | Element type | Maximum value |
| `sorted(x)` | `list[T]` | `list[T]` | New sorted list |
| `any(x)` | `list[bool]` | `bool` | True if any element is true |
| `all(x)` | `list[bool]` | `bool` | True if all elements are true |
| `map(f, x)` | `func`, `list[T]` | `list[R]` | Apply function to each element |
| `filter(p, x)` | `predicate`, `list[T]` | `list[T]` | Elements matching predicate |
| `reduce(f, x, i)` | `func`, `list[T]`, `AccT` | `AccT` | Fold left (accumulate) |
| `foldl(f, x, i)` | `func`, `list[T]`, `AccT` | `AccT` | Alias for reduce |
| `foldr(f, x, i)` | `func`, `list[T]`, `AccT` | `AccT` | Fold right-to-left |
| `range(...)` | `int` args | iterator | Integer sequence |
| `enumerate(x)` | Iterable | iterator | Index-value pairs |
| `reversed(x)` | Iterable | iterator | Reverse iteration |
| `zip(a, b)` | Two iterables | iterator | Parallel iteration |
| `chr(n)` | `int` | `str` | Unicode codepoint → string |
| `ord(s)` | `str` | `int` | String → Unicode codepoint |
| `hex(n)` | `int` | `str` | Integer → hex string (`0x...`) |
| `oct(n)` | `int` | `str` | Integer → octal string (`0o...`) |
| `bin(n)` | `int` | `str` | Integer → binary string (`0b...`) |
| `abs(n)` | `int` or `float` | Same type | Absolute value |
| `round(n, d)` | `float`, `int` | `float` | Round to d decimal places |
| `pow(b, e)` | `int`, `int` | `int` | Integer exponentiation |
| `todo()` | `str?` | never | Panic with "not implemented" |
| `hash(x)` | `Any` | `int` | FNV-1a hash of value |
| `id(x)` | `Any` | `int` | Pointer identity |

---

## Examples

All builtin function examples are provided inline throughout this document:

- **Collection Functions**: See "len", "sum", "min", "max", "sorted", "any", "all" sections
- **Transformation**: See "map", "filter", "reduce", "foldl", "foldr" sections  
- **Iteration**: See "range", "enumerate", "reversed", "zip" sections
