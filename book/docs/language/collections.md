# Collections

Desi provides three built-in collection types: **list**, **set**, and **dict**.

## List

Ordered, mutable sequence of elements.

```desi
# Create list
let numbers: list[int] = [1, 2, 3, 4, 5]
let empty: list[str] = []

# Operations
numbers.append(6)           # Add element
let first = numbers[0]      # Index access
let length = len(numbers)   # Length

# Iteration
for n: int in numbers:
    print(n)
```

### List with Custom Types

Lists can contain any type, including classes:

```desi
class Point:
    pub x: int
    pub y: int

let points: list[Point] = [
    Point(1, 2),
    Point(3, 4)
]

print(points[0].x)  # 1
```

## Set

Unordered collection of unique elements.

```desi
# Create set (use #{} syntax)
let tags: set[str] = #{"rust", "go", "desi"}
let empty: set[int] = #{}

# Operations
tags.add("python")          # Add element
let has_rust = "rust" in tags  # Membership
let count = len(tags)       # Length

# Iteration (order not guaranteed)
for tag: str in tags:
    print(tag)
```

> **Note**: Sets are unordered. Iteration order may vary.

### Set with Custom Types

Sets use **pointer identity** by default:

```desi
class Item:
    pub id: int

let i1 = Item(1)
let items: set[Item] = #{}
items.add(i1)

if i1 in items:         # Found - same pointer
    print("found")

let i2 = Item(1)
if i2 in items:         # NOT found - different pointer
    print("not found")
```

## Dict

Key-value mapping.

```desi
# Create dict
let ages: dict[str, int] = {"Alice": 30, "Bob": 25}
let empty: dict[str, int] = {}

# Operations
ages.insert("Charlie", 35)     # Insert/update
let age = ages.get("Alice", 0) # Get with default
let val = ages.setdefault("Dave", 40) # Get or insert default
let has = "Alice" in ages      # Key exists

# Iteration
for key, val: str, int in ages.items():
    print(key)
    print(val)

# Mutable iteration: modify values during iteration
let mut scores = {"alice": 80, "bob": 65}
for name, mut score in scores.items():
    if score < 70:
        score := 70  # Write-back to dict
```

### Dict with Custom Keys

Custom types can be dict keys. By default, they use **pointer identity**:

```desi
class Point:
    pub x: int
    pub y: int

let p = Point(1, 2)
let cache: dict[Point, str] = {}
cache.insert(p, "origin")

if p in cache:      # Found - same pointer
    print("found")
```

For **value-based** lookup, implement `__hash__` and `__eq__`:

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __hash__(self) -> u64:
        return self.x * 31 + self.y
    
    pub def __eq__(self, other: Point) -> bool:
        return self.x == other.x and self.y == other.y

let cache: dict[Point, str] = {Point(1, 2): "origin"}

# Different object, same values - FOUND!
if Point(1, 2) in cache:
    print("found via value equality")
```

## Nested Collections

Collections can be nested arbitrarily:

```desi
# List of dicts
let records: list[dict[str, int]] = []
records.append({"x": 100, "y": 200})
print(records[0].get("x", 0))  # 100

# Dict with list values
let groups: dict[str, list[int]] = {}
groups.insert("evens", [2, 4, 6])
let evens = groups.get("evens", [])
print(evens[0])  # 2

# Deeply nested
let matrix: list[list[set[int]]] = []
# ... build structure ...
```

## Memory Ownership

- Collections **own** their elements
- `dict.get` returns a **borrowed reference** (not a copy)
- When a collection is freed, its elements are freed

```desi
let cache: dict[str, Item] = {}
cache.insert("key", Item(42))

let item = cache.get("key", Item(0))  # Borrowed reference
print(item.id)  # 42
# item is NOT freed here - dict owns it
```

## See Also

- [Types](types.md) - Type system overview
- [Iteration](iteration.md) - For-loop patterns
- [Classes](classes.md) - Custom types
