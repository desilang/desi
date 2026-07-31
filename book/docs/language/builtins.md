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
print(1, true, 3.14, "text")  # → 1 true 3.14 text

# Floats always show decimal point (Python-like)
print(5.0)  # → 5.0 (not 5)

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
- Custom classes with `__format__`, `__repr__`, or `__str__` methods

### Custom Formatting in F-Strings

Define `__format__` for custom class formatting:

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __format__(self, spec: str) -> str:
        if spec == "short":
            return f"({self.x},{self.y})"
        return f"Point({self.x}, {self.y})"

let p = Point(x=1, y=2)
print(f"{p:short}")  # → (1,2)
print(f"{p}")        # → Point(1, 2)
```

**Fallback Chain:** When displaying a custom class in f-strings or `print()`:
1. `__format__(self, spec)` - Primary (receives format specifier)
2. `__repr__(self)` - Fallback (no specifier support)
3. `<?>` - Default for types without either method

**Example with `__repr__` fallback:**
```desi
class Vector:
    pub x: int
    pub y: int
    
    pub def __repr__(self) -> str:
        return f"Vector<{self.x}, {self.y}>"

let v = Vector(x=3, y=4)
print(f"{v}")  # → Vector<3, 4> (uses __repr__)
```

**Dos:**
- ✅ Use `__format__` for types that need format specifiers
- ✅ Use `__repr__` for simple string representation
- ✅ Return a `str` from both methods

**Don'ts:**
- ❌ Don't define format specs that conflict with built-in types (`:d`, `:f`, etc.)
- ❌ Don't forget the `spec` parameter in `__format__` signature

---

### dbg()

Debug print that shows file, line, expression, and value:

```desi
let x = 42
dbg(x)  # → [myfile.desi:2] x = 42

let result = dbg(x * 2)  # → [myfile.desi:4] x * 2 = 84
print(result)  # → 84
```

**Key Features:**
- Shows source file and line number
- Shows the expression text (not just the value)
- Returns the value (can be used in assignments/expressions)
- Works with any type

**Use Cases:**
- Quick debugging without modifying code structure
- Inspecting intermediate values in pipelines
- Tracking code execution with location info

```desi
# Chaining - inspect intermediate values
let result = dbg(calculate(input))

# In expressions
let doubled = dbg(x) * 2  # prints x, then multiplies

# Pipeline debugging
let value = process(dbg(fetch(url)))
```

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

# Reverse sort
let descending: list[int] = sorted(nums, reverse=true)

# With pipeline
let piped: list[int] = nums |> sorted(reverse=true)
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
for i in range(5):
    print(str(i))  # 0, 1, 2, 3, 4

for i in range(2, 6):
    print(str(i))  # 2, 3, 4, 5

for i in range(0, 10, 2):
    print(str(i))  # 0, 2, 4, 6, 8

# A negative step counts down
for i in range(3, 0, -1):
    print(str(i))  # 3, 2, 1
```

A range is a value, not just loop syntax. It holds only its start, stop and
step, so it never materialises its elements — `range(1000000000)` costs the
same as `range(3)`:

```desi
let r = range(0, 10, 2)

print(str(len(r)))   # 5
print(str(r[2]))     # 4 — computed, not stored

if 8 in r:           # arithmetic, not a scan
    print("yes")

let squares = [x * x for x in r]
```

The type is written `range`, so a function can take one:

```desi
def total(r: range) -> int:
    let mut sum = 0
    for v in r:
        sum := sum + v
    return sum
```

!!! note "The step may not be zero"
    A zero step would never terminate, so it is treated as `1`.

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

> **Note**: If the lists have different lengths, `zip()` truncates to the shortest.
> For example, `zip([1,2,3], [10,20])` iterates only 2 times.

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
| `range(...)` | Lazy int sequence — iterable, indexable, `len()` |
| `enumerate(x)` | Index-value pairs |
| `reversed(x)` | Reverse iteration |
| `zip(a, b)` | Parallel iteration |
