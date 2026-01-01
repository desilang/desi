# Built-in Functions

Desi provides Python-like built-in functions for I/O, collections, and iteration.

## I/O Functions

### print()

Output values to the console with automatic formatting:

```desi
# Single argument
print("Hello, World!")

# Multiple arguments (separated by spaces)
print("Name:", name, "Age:", age)

# Mixed types - automatic conversion
print(1, true, 3.14, "text")  # → 1 true 3.140000 text

# No arguments - prints empty line
print()
```

**Keyword Arguments:**

| Param | Default | Description |
|-------|---------|-------------|
| `sep` | `" "` | String inserted between arguments |
| `end` | `"\n"` | String appended after the last argument |
| `file` | `sys.stdout` | Output stream (`sys.stdout` or `sys.stderr`) |
| `flush` | `false` | Force immediate output (no buffering) |
| `style` | `""` | ANSI color/style: `red`, `green`, `bold`, etc. |

```desi
# Custom separator
print("a", "b", "c", sep="-")  # → a-b-c

# Custom end (no newline)
print("loading", end="")
print("...")  # → loading...

# Both sep and end
print("x", "y", sep=",", end="!\n")  # → x,y!

# Write to stderr
print("Error: file not found", file=sys.stderr)

# Write to a file
let f = open("log.txt", "w")
print("Log entry 1", file=f)
print("Log entry 2", file=f)
f.close()

# With auto-close (recommended)
using f = open("data.txt", "w"):
    print("Auto-closed!", file=f)
    # No need to call f.close()

# Force immediate output
print("Progress: 50%", flush=true)

# Colored output
print("Error!", style="red")
print("Success!", style="green")
print("Warning:", style="bold,yellow")
```

**Supported Types:**
- Strings, integers, floats, booleans
- Custom classes with `__str__` or `__repr__` methods
- Collections with Display trait implementation

---

## Log Module

Structured logging with colored prefixes and automatic stream routing.

```desi
log.info("Server started on port 8080")
log.debug("Loading configuration...")
log.warn("Disk space running low")
log.error("Connection refused")
```

| Function | Prefix | Stream | Color |
|----------|--------|--------|-------|
| `log.info()` | `[INFO]` | stdout | green |
| `log.debug()` | `[DEBUG]` | stdout | dim |
| `log.warn()` | `[WARN]` | stderr | yellow |
| `log.error()` | `[ERROR]` | stderr | red |

---

## Collection Functions

### len()

Get the length of a collection or string:

```desi
let items: list[int] = [1, 2, 3, 4, 5]
print(len(items))  # 5

let name: str = "Desi"
print(len(name))   # 4
```

### sum()

Add all numbers in a list:

```desi
let numbers: list[int] = [1, 2, 3, 4, 5]
let total: int = sum(numbers)  # 15
```

### min() / max()

Find the smallest or largest value:

```desi
let numbers: list[int] = [3, 1, 4, 1, 5, 9]
print(min(numbers))  # 1
print(max(numbers))  # 9
```

### sorted()

Return a new sorted list (original unchanged):

```desi
let nums: list[int] = [3, 1, 4, 1, 5]
let ordered: list[int] = sorted(nums)
# ordered = [1, 1, 3, 4, 5]
# nums = [3, 1, 4, 1, 5] (unchanged)
```

### any() / all()

Check boolean conditions across a list:

```desi
let flags: list[bool] = [true, false, true]

if any(flags):
    print("At least one is true")

if all(flags):
    print("All are true")  # Won't print
```

---

## Transformation Functions

### map()

Apply a function to each element:

```desi
let numbers: list[int] = [1, 2, 3, 4, 5]
let doubled: list[int] = map(lambda<int> x: int: x * 2, numbers)
# doubled = [2, 4, 6, 8, 10]

# Method style
let squared: list[int] = numbers.map(lambda<int> x: int: x * x)
```

### filter()

Keep elements that match a condition:

```desi
let numbers: list[int] = [1, 2, 3, 4, 5, 6]
let evens: list[int] = filter(lambda<bool> x: int: x % 2 == 0, numbers)
# evens = [2, 4, 6]

# Method style
let odds: list[int] = numbers.filter(lambda<bool> x: int: x % 2 != 0)
```

### reduce()

Combine all elements into one value:

```desi
def add(acc: int, x: int) -> int:
    return acc + x

let nums: list[int] = [1, 2, 3, 4, 5]
let total: int = reduce(add, nums, 0)  # 15
```

---

## Iteration Functions

### range()

Generate a sequence of integers:

```desi
for i: int in range(5):
    print(i)  # 0, 1, 2, 3, 4

for i: int in range(2, 6):
    print(i)  # 2, 3, 4, 5

for i: int in range(0, 10, 2):
    print(i)  # 0, 2, 4, 6, 8
```

### enumerate()

Get index and value together:

```desi
let items: list[str] = ["a", "b", "c"]
for i: int, item: str in enumerate(items):
    print(f"{i}: {item}")
# 0: a
# 1: b
# 2: c
```

### reversed()

Iterate in reverse order:

```desi
let items: list[int] = [1, 2, 3]
for x: int in reversed(items):
    print(x)  # 3, 2, 1
```

### zip()

Pair elements from two lists:

```desi
let names: list[str] = ["alice", "bob"]
let scores: list[int] = [100, 85]

for name: str, score: int in zip(names, scores):
    print(f"{name}: {score}")
# alice: 100
# bob: 85
```

---

## Quick Reference

| Function | Description |
|----------|-------------|
| `print(...)` | Output to console |
| `len(x)` | Number of elements |
| `sum(x)` | Sum of elements |
| `min(x)` / `max(x)` | Min/max value |
| `sorted(x)` | New sorted list |
| `any(x)` / `all(x)` | Boolean checks |
| `map(f, x)` | Transform elements |
| `filter(f, x)` | Filter elements |
| `reduce(f, x, init)` | Combine elements |
| `range(...)` | Integer sequence |
| `enumerate(x)` | Index-value pairs |
| `reversed(x)` | Reverse iteration |
| `zip(a, b)` | Parallel iteration |
