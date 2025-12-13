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

```desi
let numbers: list[int] = [3, 1, 4, 1, 5, 9, 2, 6]
let ordered: list[int] = sorted(numbers)
# ordered = [1, 1, 2, 3, 4, 5, 6, 9]
# numbers = [3, 1, 4, 1, 5, 9, 2, 6] (unchanged)
```

**Accepted Types:**
- `list[int]` → `list[int]`

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

# DON'T: Expect sorted to work on non-int lists (currently)
let names: list[str] = ["bob", "alice"]
sorted(names)  # ❌ Error: sorted requires list[int]

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
| `sorted(x)` | `list[int]` | `list[int]` | New sorted list |
| `any(x)` | `list[bool]` | `bool` | True if any element is true |
| `all(x)` | `list[bool]` | `bool` | True if all elements are true |
| `range(...)` | `int` args | iterator | Integer sequence |
| `enumerate(x)` | Iterable | iterator | Index-value pairs |
| `reversed(x)` | Iterable | iterator | Reverse iteration |
| `zip(a, b)` | Two iterables | iterator | Parallel iteration |

---

## Examples

- `examples/153_builtins.desi` - sum, min, max
- `examples/154_reversed.desi` - reversed iteration
- `examples/156_any_all_sorted.desi` - any, all, sorted
- `examples/152_enumerate.desi` - enumerate
- `examples/156_zip.desi` - zip
